using System.Net;
using System.Text;
using System.Text.Json;
using KvmSwitcher;
using Xunit;

namespace KvmSwitcher.Tests;

/// <summary>Exercises KvmClient against the shared fixtures in &lt;repo&gt;/fixtures (see docs/API.md).</summary>
public sealed class KvmClientTests
{
    private const string TestPassword = "correct horse";
    private static readonly string FixturesDirectory = LocateFixturesDirectory();

    private readonly AppSettings testSettings = new() { Host = "kvm.test" };

    // ---- Helpers ----

    private static string LocateFixturesDirectory()
    {
        for (var searchDirectory = new DirectoryInfo(AppContext.BaseDirectory);
             searchDirectory is not null;
             searchDirectory = searchDirectory.Parent)
        {
            string candidate = Path.Combine(searchDirectory.FullName, "fixtures");
            if (File.Exists(Path.Combine(candidate, "cases.json")))
            {
                return candidate;
            }
        }

        throw new DirectoryNotFoundException("Could not find the repository's fixtures directory.");
    }

    private static string Fixture(string fileName) => File.ReadAllText(Path.Combine(FixturesDirectory, fileName));

    private static StubHttpHandler FixtureKvm(
        string switchFixture = "switch_state.json",
        string configFixture = "get_config.json",
        Dictionary<string, (HttpStatusCode, string)>? overrides = null)
    {
        var routes = new Dictionary<string, (HttpStatusCode, string)>
        {
            ["/api/auth/login"] = (HttpStatusCode.OK, "login_success.json"),
            ["/api/switch"] = (HttpStatusCode.OK, switchFixture),
            ["/api/system/get_config"] = (HttpStatusCode.OK, configFixture),
            ["/api/switch/set_active"] = (HttpStatusCode.OK, "set_active_success.json"),
        };
        foreach ((string path, (HttpStatusCode, string) route) in overrides ?? new())
        {
            routes[path] = route;
        }

        return new StubHttpHandler(request =>
            routes.TryGetValue(request.RequestUri!.AbsolutePath, out var route)
                ? (route.Item1, Fixture(route.Item2))
                : (HttpStatusCode.NotFound, ""));
    }

    private KvmClient MakeClient(StubHttpHandler stubHandler, string? password = TestPassword)
    {
        return new KvmClient(testSettings, () => password, stubHandler);
    }

    // ---- Shared contract cases ----

    [Fact]
    public async Task SharedFixtureCasesParseAsExpected()
    {
        using JsonDocument contract = JsonDocument.Parse(Fixture("cases.json"));
        JsonElement contractCases = contract.RootElement.GetProperty("cases");
        Assert.True(contractCases.GetArrayLength() > 0);

        foreach (JsonElement contractCase in contractCases.EnumerateArray())
        {
            string caseName = contractCase.GetProperty("name").GetString()!;
            using var stubHandler = FixtureKvm(
                contractCase.GetProperty("switch").GetString()!,
                contractCase.GetProperty("config").GetString()!);
            KvmSwitchState switchState = await MakeClient(stubHandler).FetchSwitchStateAsync();

            JsonElement expected = contractCase.GetProperty("expected");
            JsonElement[] expectedPorts = expected.GetProperty("ports").EnumerateArray().ToArray();
            Assert.True(expected.GetProperty("active_port_index").GetInt32() == switchState.ActivePortIndex, caseName);
            Assert.Equal(
                expectedPorts.Select(port => port.GetProperty("display_name").GetString()),
                switchState.Ports.Select(port => port.DisplayName(testSettings)));
            Assert.Equal(
                expectedPorts.Select(port => port.GetProperty("has_video_link").GetBoolean()),
                switchState.Ports.Select(port => port.HasVideoLink));
        }
    }

    // ---- Authentication ----

    [Fact]
    public async Task LogsInOnceAndSendsTokenHeader()
    {
        using var stubHandler = FixtureKvm();
        var client = MakeClient(stubHandler);
        await client.FetchSwitchStateAsync();
        await client.FetchSwitchStateAsync();

        Assert.Single(stubHandler.RequestsTo("/api/auth/login"));
        var switchRequests = stubHandler.RequestsTo("/api/switch");
        Assert.Equal(2, switchRequests.Count);
        Assert.Equal(new string('0', 64), switchRequests[0].TokenHeader);
    }

    [Fact]
    public async Task LoginFormEncodesSpecialCharacters()
    {
        using var stubHandler = FixtureKvm();
        await MakeClient(stubHandler, "p&ss+word= ü").FetchSwitchStateAsync();

        RecordedRequest loginRequest = Assert.Single(stubHandler.RequestsTo("/api/auth/login"));
        Assert.Equal(HttpMethod.Post, loginRequest.Method);
        // FormUrlEncodedContent encodes spaces as "+", which is valid form encoding.
        Assert.Equal("user=admin&passwd=p%26ss%2Bword%3D+%C3%BC", loginRequest.Body);
    }

    [Fact]
    public async Task ExpiredTokenTriggersOneReloginAndRetry()
    {
        int switchCallCount = 0;
        using var stubHandler = new StubHttpHandler(request => request.RequestUri!.AbsolutePath switch
        {
            "/api/auth/login" => (HttpStatusCode.OK, Fixture("login_success.json")),
            "/api/system/get_config" => (HttpStatusCode.OK, Fixture("get_config.json")),
            "/api/switch" => ++switchCallCount == 1
                ? (HttpStatusCode.Unauthorized, Fixture("error_unauthorized.json"))
                : (HttpStatusCode.OK, Fixture("switch_state.json")),
            _ => (HttpStatusCode.NotFound, ""),
        });

        KvmSwitchState switchState = await MakeClient(stubHandler).FetchSwitchStateAsync();
        Assert.Equal(2, switchState.ActivePortIndex);
        Assert.Equal(2, stubHandler.RequestsTo("/api/auth/login").Count);
    }

    [Fact]
    public async Task WrongPasswordReportsLoginRejected()
    {
        using var stubHandler = FixtureKvm(overrides: new()
        {
            ["/api/auth/login"] = (HttpStatusCode.Forbidden, "error_forbidden.json"),
        });
        var loginError = await Assert.ThrowsAsync<KvmException>(() => MakeClient(stubHandler).FetchSwitchStateAsync());
        Assert.Equal(KvmException.LoginRejected, loginError.Message);
    }

    [Fact]
    public async Task MissingPasswordReportsNotConfiguredWithoutNetwork()
    {
        using var stubHandler = FixtureKvm();
        var configError = await Assert.ThrowsAsync<KvmException>(
            () => MakeClient(stubHandler, password: null).FetchSwitchStateAsync());
        Assert.Equal(KvmException.NotConfigured, configError.Message);
        Assert.Empty(stubHandler.Requests);
    }

    [Fact]
    public async Task EmptyHostReportsNotConfigured()
    {
        testSettings.Host = "";
        using var stubHandler = FixtureKvm();
        var configError = await Assert.ThrowsAsync<KvmException>(() => MakeClient(stubHandler).FetchSwitchStateAsync());
        Assert.Equal(KvmException.NotConfigured, configError.Message);
    }

    // ---- Switching and names ----

    [Fact]
    public async Task SetActivePortSendsZeroBasedIndex()
    {
        using var stubHandler = FixtureKvm();
        await MakeClient(stubHandler).SetActivePortAsync(3);

        RecordedRequest setActiveRequest = Assert.Single(stubHandler.RequestsTo("/api/switch/set_active"));
        Assert.Equal(HttpMethod.Post, setActiveRequest.Method);
        Assert.Equal("?port=3", setActiveRequest.Query);
    }

    [Fact]
    public async Task ConfigFailureFallsBackToDefaultNames()
    {
        using var stubHandler = FixtureKvm(overrides: new()
        {
            ["/api/system/get_config"] = (HttpStatusCode.InternalServerError, "error_forbidden.json"),
        });
        KvmSwitchState switchState = await MakeClient(stubHandler).FetchSwitchStateAsync();
        Assert.Equal(
            new[] { "Port 1", "Port 2", "Port 3", "Port 4" },
            switchState.Ports.Select(port => port.DisplayName(testSettings)));
    }

    [Fact]
    public async Task LocalNamesFillGapsButKvmNamesWin()
    {
        testSettings.LocalPortNames = new() { "Local One", "", "Local Three" };
        using (var stubHandler = FixtureKvm(configFixture: "get_config_without_port_names.json"))
        {
            KvmSwitchState switchState = await MakeClient(stubHandler).FetchSwitchStateAsync();
            Assert.Equal(
                new[] { "Local One", "Port 2", "Local Three", "Port 4" },
                switchState.Ports.Select(port => port.DisplayName(testSettings)));
        }

        using (var stubHandler = FixtureKvm())
        {
            KvmSwitchState namedState = await MakeClient(stubHandler).FetchSwitchStateAsync();
            Assert.Equal("Workstation", namedState.Ports[0].DisplayName(testSettings));
        }
    }

    [Fact]
    public async Task ApiErrorMessageIsSurfaced()
    {
        using var stubHandler = FixtureKvm(overrides: new()
        {
            ["/api/switch/set_active"] = (HttpStatusCode.OK, "error_forbidden.json"),
        });
        var apiError = await Assert.ThrowsAsync<KvmException>(() => MakeClient(stubHandler).SetActivePortAsync(1));
        Assert.Equal("KVM error: Forbidden", apiError.Message);
    }

    // ---- Settings ----

    [Theory]
    [InlineData("  https://glkvm.local/ ", "glkvm.local")]
    [InlineData("HTTP://10.0.0.5//", "10.0.0.5")]
    [InlineData("glkvm.local", "glkvm.local")]
    public void NormalizeHostStripsSchemeAndSlashes(string rawHost, string expectedHost)
    {
        Assert.Equal(expectedHost, AppSettings.NormalizeHost(rawHost));
    }
}

internal sealed record RecordedRequest(HttpMethod Method, string Path, string Query, string? TokenHeader, string Body);

/// <summary>Answers HttpClient requests from a test-supplied function instead of the network.</summary>
internal sealed class StubHttpHandler(Func<HttpRequestMessage, (HttpStatusCode, string)> respond) : HttpMessageHandler
{
    private readonly List<RecordedRequest> recordedRequests = new();

    public IReadOnlyList<RecordedRequest> Requests => recordedRequests;

    public List<RecordedRequest> RequestsTo(string path) =>
        recordedRequests.Where(recorded => recorded.Path == path).ToList();

    protected override async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request, CancellationToken cancellationToken)
    {
        string requestBody = request.Content is null ? "" : await request.Content.ReadAsStringAsync(cancellationToken);
        string? tokenHeader = request.Headers.TryGetValues("token", out var tokenValues) ? tokenValues.First() : null;
        recordedRequests.Add(new RecordedRequest(
            request.Method, request.RequestUri!.AbsolutePath, request.RequestUri.Query, tokenHeader, requestBody));

        (HttpStatusCode statusCode, string responseBody) = respond(request);
        return new HttpResponseMessage(statusCode)
        {
            Content = new StringContent(responseBody, Encoding.UTF8, "application/json"),
        };
    }
}
