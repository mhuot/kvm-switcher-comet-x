using System.Net;
using System.Net.Security;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text.Json;

namespace KvmSwitcher;

/// <summary>One downstream port on the GL.iNet KVM's built-in switch.</summary>
internal sealed record KvmPort(int PortIndex, string PortId, string DeviceName, bool HasVideoLink)
{
    public string DisplayName(AppSettings settings)
    {
        if (DeviceName.Length > 0)
        {
            return DeviceName;
        }

        string localName = settings.LocalName(PortIndex);
        return localName.Length > 0 ? localName : $"Port {PortIndex + 1}";
    }
}

internal sealed record KvmSwitchState(int ActivePortIndex, IReadOnlyList<KvmPort> Ports);

/// <summary>An error with a message suitable for showing directly to the user.</summary>
internal sealed class KvmException(string userMessage) : Exception(userMessage)
{
    public const string NotConfigured = "Set the KVM address and password in Settings.";
    public const string LoginRejected = "Login rejected. Check the password in Settings.";
    public const string TwoFactorRequired = "2FA is enabled on the KVM; this app does not support it.";
    public const string CertificateChanged =
        "The KVM's certificate changed. Reset the pinned certificate in Settings if expected.";
    public const string UnexpectedResponse = "Unexpected response from the KVM.";
}

/// <summary>
/// Talks to the GLKVM (PiKVM-derived kvmd) HTTP API. Every reply is {"ok": bool, "result": …}.
///   POST /api/auth/login               form: user=admin&amp;passwd=…  -> { token }
///   GET  /api/switch                   header token: …            -> switch state
///   POST /api/switch/set_active?port=N (N is zero-based)
///   GET  /api/system/get_config        -> config.port_names (names set in the web UI)
/// </summary>
internal sealed class KvmClient : IDisposable
{
    private const string AdminUserName = "admin";

    private readonly AppSettings settings;
    private readonly HttpClient httpClient;
    private string? sessionToken;
    private volatile bool certificateMismatch;

    public KvmClient(AppSettings settings)
    {
        this.settings = settings;
        var httpHandler = new SocketsHttpHandler
        {
            SslOptions = new SslClientAuthenticationOptions
            {
                RemoteCertificateValidationCallback = ValidateServerCertificate,
            },
            PooledConnectionLifetime = TimeSpan.FromMinutes(2),
        };
        httpClient = new HttpClient(httpHandler) { Timeout = TimeSpan.FromSeconds(8) };
    }

    public void ForgetSession()
    {
        sessionToken = null;
    }

    public async Task<KvmSwitchState> FetchSwitchStateAsync()
    {
        JsonElement switchResult = await AuthorizedRequestAsync(HttpMethod.Get, "api/switch");

        // Names are cosmetic, so a failed config read should not block switching.
        IReadOnlyList<string> webUIPortNames;
        try
        {
            webUIPortNames = await FetchWebUIPortNamesAsync();
        }
        catch (KvmException)
        {
            webUIPortNames = Array.Empty<string>();
        }

        return ParseSwitchState(switchResult, webUIPortNames);
    }

    public async Task SetActivePortAsync(int portIndex)
    {
        await AuthorizedRequestAsync(HttpMethod.Post, $"api/switch/set_active?port={portIndex}");
    }

    public void Dispose()
    {
        httpClient.Dispose();
    }

    /// <summary>The GL.iNet web UI saves port names in its own config, not on the kvmd switch ports.</summary>
    private async Task<IReadOnlyList<string>> FetchWebUIPortNamesAsync()
    {
        JsonElement configResult = await AuthorizedRequestAsync(HttpMethod.Get, "api/system/get_config");
        if (configResult.TryGetProperty("config", out JsonElement webUIConfig)
            && webUIConfig.TryGetProperty("port_names", out JsonElement portNamesElement)
            && portNamesElement.ValueKind == JsonValueKind.Array)
        {
            return portNamesElement.EnumerateArray().Select(nameElement => nameElement.GetString() ?? "").ToList();
        }

        return Array.Empty<string>();
    }

    // ---- Request plumbing ----

    private Uri BuildUri(string relativePath)
    {
        if (string.IsNullOrWhiteSpace(settings.Host)
            || !Uri.TryCreate($"https://{settings.Host}/{relativePath}", UriKind.Absolute, out Uri? requestUri))
        {
            throw new KvmException(KvmException.NotConfigured);
        }

        return requestUri;
    }

    /// <summary>Sends a request with the cached token, logging in first or again after a 401/403.</summary>
    private async Task<JsonElement> AuthorizedRequestAsync(HttpMethod method, string relativePath)
    {
        sessionToken ??= await LoginAsync();

        using (HttpResponseMessage firstResponse = await SendAsync(BuildAuthorizedRequest(method, relativePath)))
        {
            if (firstResponse.StatusCode is not (HttpStatusCode.Unauthorized or HttpStatusCode.Forbidden))
            {
                return await UnwrapResultAsync(firstResponse);
            }
        }

        sessionToken = await LoginAsync();
        using HttpResponseMessage retryResponse = await SendAsync(BuildAuthorizedRequest(method, relativePath));
        return await UnwrapResultAsync(retryResponse);
    }

    private HttpRequestMessage BuildAuthorizedRequest(HttpMethod method, string relativePath)
    {
        var request = new HttpRequestMessage(method, BuildUri(relativePath));
        request.Headers.Add("token", sessionToken);
        request.Headers.Accept.ParseAdd("application/json");
        return request;
    }

    private async Task<string> LoginAsync()
    {
        string? password = CredentialStore.ReadPassword();
        if (string.IsNullOrEmpty(password))
        {
            throw new KvmException(KvmException.NotConfigured);
        }

        var loginRequest = new HttpRequestMessage(HttpMethod.Post, BuildUri("api/auth/login"))
        {
            Content = new FormUrlEncodedContent(new Dictionary<string, string>
            {
                ["user"] = AdminUserName,
                ["passwd"] = password,
            }),
        };

        using HttpResponseMessage loginResponse = await SendAsync(loginRequest);
        if (loginResponse.StatusCode is HttpStatusCode.Unauthorized or HttpStatusCode.Forbidden)
        {
            throw new KvmException(KvmException.LoginRejected);
        }

        JsonElement loginResult = await UnwrapResultAsync(loginResponse);
        if (loginResult.ValueKind == JsonValueKind.Object
            && loginResult.TryGetProperty("token", out JsonElement tokenElement)
            && tokenElement.GetString() is { Length: > 0 } token)
        {
            return token;
        }

        if (loginResult.ValueKind == JsonValueKind.Object
            && (loginResult.TryGetProperty("need_2fa", out _) || loginResult.TryGetProperty("2fa", out _)))
        {
            throw new KvmException(KvmException.TwoFactorRequired);
        }

        throw new KvmException(KvmException.UnexpectedResponse);
    }

    private async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request)
    {
        certificateMismatch = false;
        try
        {
            using (request)
            {
                return await httpClient.SendAsync(request);
            }
        }
        catch (HttpRequestException) when (certificateMismatch)
        {
            throw new KvmException(KvmException.CertificateChanged);
        }
        catch (HttpRequestException requestError)
        {
            throw new KvmException($"Could not reach the KVM: {requestError.Message}");
        }
        catch (TaskCanceledException)
        {
            throw new KvmException("The KVM did not respond in time.");
        }
    }

    /// <summary>kvmd wraps every reply as {"ok": bool, "result": …}; returns "result".</summary>
    private static async Task<JsonElement> UnwrapResultAsync(HttpResponseMessage response)
    {
        string responseText = await response.Content.ReadAsStringAsync();
        JsonElement envelope;
        try
        {
            using JsonDocument responseDocument = JsonDocument.Parse(responseText);
            envelope = responseDocument.RootElement.Clone();
        }
        catch (JsonException)
        {
            throw new KvmException(response.IsSuccessStatusCode
                ? KvmException.UnexpectedResponse
                : $"KVM error: HTTP {(int)response.StatusCode}");
        }

        if (envelope.ValueKind != JsonValueKind.Object)
        {
            throw new KvmException(KvmException.UnexpectedResponse);
        }

        bool requestSucceeded = envelope.TryGetProperty("ok", out JsonElement okElement)
            && okElement.ValueKind == JsonValueKind.True;
        envelope.TryGetProperty("result", out JsonElement resultElement);

        if (!requestSucceeded)
        {
            string errorMessage = resultElement.ValueKind == JsonValueKind.Object
                && resultElement.TryGetProperty("error_msg", out JsonElement errorElement)
                    ? errorElement.GetString() ?? ""
                    : $"HTTP {(int)response.StatusCode}";
            throw new KvmException($"KVM error: {errorMessage}");
        }

        return resultElement;
    }

    private static KvmSwitchState ParseSwitchState(JsonElement switchResult, IReadOnlyList<string> webUIPortNames)
    {
        try
        {
            int activePortIndex = switchResult.GetProperty("summary").GetProperty("active_port").GetInt32();
            JsonElement rawPorts = switchResult.GetProperty("model").GetProperty("ports");

            var videoLinks = new List<bool>();
            if (switchResult.TryGetProperty("video", out JsonElement videoElement)
                && videoElement.TryGetProperty("links", out JsonElement linksElement)
                && linksElement.ValueKind == JsonValueKind.Array)
            {
                videoLinks.AddRange(linksElement.EnumerateArray().Select(linkElement => linkElement.ValueKind == JsonValueKind.True));
            }

            var ports = rawPorts.EnumerateArray().Select((rawPort, portIndex) =>
            {
                string switchPortName = rawPort.TryGetProperty("name", out JsonElement nameElement)
                    ? (nameElement.GetString() ?? "").Trim()
                    : "";
                string webUIPortName = portIndex < webUIPortNames.Count ? webUIPortNames[portIndex].Trim() : "";
                string portId = rawPort.TryGetProperty("id", out JsonElement idElement)
                    ? idElement.GetString() ?? ""
                    : $"{portIndex + 1}";
                return new KvmPort(
                    portIndex,
                    portId,
                    switchPortName.Length > 0 ? switchPortName : webUIPortName,
                    portIndex >= videoLinks.Count || videoLinks[portIndex]);
            }).ToList();

            return new KvmSwitchState(activePortIndex, ports);
        }
        catch (Exception parseError) when (parseError is KeyNotFoundException or InvalidOperationException or FormatException)
        {
            throw new KvmException(KvmException.UnexpectedResponse);
        }
    }

    // ---- TLS: trust-on-first-use pinning of the KVM's self-signed certificate ----

    private bool ValidateServerCertificate(
        object sender, X509Certificate? certificate, X509Chain? chain, SslPolicyErrors policyErrors)
    {
        if (certificate is null)
        {
            return false;
        }

        // This HttpClient only ever talks to the configured KVM, so every certificate seen is the KVM's.
        string fingerprint = Convert.ToHexString(SHA256.HashData(certificate.GetRawCertData())).ToLowerInvariant();
        string? pinnedFingerprint = settings.PinnedCertificateFingerprint;
        if (pinnedFingerprint is null)
        {
            settings.PinnedCertificateFingerprint = fingerprint;
            settings.Save();
            return true;
        }

        if (pinnedFingerprint == fingerprint)
        {
            return true;
        }

        certificateMismatch = true;
        return false;
    }
}
