using System.Runtime.InteropServices;
using ComTypes = System.Runtime.InteropServices.ComTypes;

namespace KvmSwitcher;

/// <summary>
/// Stores the KVM admin password in Windows Credential Manager (a generic credential),
/// the Windows counterpart of the macOS Keychain.
/// </summary>
internal static class CredentialStore
{
    public const string TargetName = "KVMSwitcher/glkvm-admin";
    private const string UserName = "admin";
    private const int CredentialTypeGeneric = 1;
    private const int CredentialPersistLocalMachine = 2;

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    private struct NativeCredential
    {
        public int Flags;
        public int Type;
        public string TargetName;
        public string? Comment;
        public ComTypes.FILETIME LastWritten;
        public int CredentialBlobSize;
        public IntPtr CredentialBlob;
        public int Persist;
        public int AttributeCount;
        public IntPtr Attributes;
        public string? TargetAlias;
        public string UserName;
    }

    [DllImport("advapi32.dll", EntryPoint = "CredReadW", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredRead(string target, int type, int reservedFlag, out IntPtr credentialPointer);

    [DllImport("advapi32.dll", EntryPoint = "CredWriteW", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredWrite(ref NativeCredential credential, int flags);

    [DllImport("advapi32.dll", EntryPoint = "CredDeleteW", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern bool CredDelete(string target, int type, int flags);

    [DllImport("advapi32.dll")]
    private static extern void CredFree(IntPtr buffer);

    public static string? ReadPassword(string targetName = TargetName)
    {
        if (!CredRead(targetName, CredentialTypeGeneric, 0, out IntPtr credentialPointer))
        {
            return null;
        }

        try
        {
            var storedCredential = Marshal.PtrToStructure<NativeCredential>(credentialPointer);
            if (storedCredential.CredentialBlob == IntPtr.Zero || storedCredential.CredentialBlobSize == 0)
            {
                return null;
            }

            return Marshal.PtrToStringUni(storedCredential.CredentialBlob, storedCredential.CredentialBlobSize / 2);
        }
        finally
        {
            CredFree(credentialPointer);
        }
    }

    /// <param name="targetName">Overridable so the self-test never touches the real credential.</param>
    public static void SavePassword(string? newPassword, string targetName = TargetName)
    {
        if (string.IsNullOrEmpty(newPassword))
        {
            CredDelete(targetName, CredentialTypeGeneric, 0);
            return;
        }

        IntPtr passwordBlob = Marshal.StringToCoTaskMemUni(newPassword);
        try
        {
            var newCredential = new NativeCredential
            {
                Type = CredentialTypeGeneric,
                TargetName = targetName,
                CredentialBlob = passwordBlob,
                CredentialBlobSize = newPassword.Length * 2,
                Persist = CredentialPersistLocalMachine,
                UserName = UserName,
            };
            if (!CredWrite(ref newCredential, 0))
            {
                throw new InvalidOperationException(
                    $"Could not save the password to Credential Manager (error {Marshal.GetLastWin32Error()}).");
            }
        }
        finally
        {
            Marshal.ZeroFreeCoTaskMemUnicode(passwordBlob);
        }
    }
}
