# Feed-System

A Go short-video backend learning project. The current code supports account registration and login, JWT authentication, current-user lookup, video metadata publishing, public video listing and detail, and video likes.

## Requirements

- Go 1.27 or later
- A Base64-encoded JWT secret representing at least 32 random bytes

## Local setup

1. Create your local configuration file:

   ```powershell
   Copy-Item configs/local.example.yaml configs/local.yaml
   ```

2. In the PowerShell window that will run the API, create a local JWT secret:

   ```powershell
   $secretBytes = New-Object byte[] 32
   [System.Security.Cryptography.RandomNumberGenerator]::Fill($secretBytes)
   $env:CLIPFLOW_JWT_SECRET = [Convert]::ToBase64String($secretBytes)
   ```

3. Start the API:

   ```powershell
   .\run.ps1
   ```

The API listens on the address in `configs/local.yaml`. Runtime data is created under `.run/` and is intentionally not committed.

## Verification

```powershell
curl.exe -i http://127.0.0.1:18080/healthz
```

## Learning documentation

See `doc/` for the staged teaching notes and `doc/plan.md` for the overall plan.