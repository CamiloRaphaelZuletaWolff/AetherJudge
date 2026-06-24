# Creates local .env files from their examples (idempotent).
# Invoked by `task env:init` on Windows; see env-init.sh for the Unix twin.
$ErrorActionPreference = "Stop"

if (-not (Test-Path .env)) {
    $rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
    $bytes = New-Object byte[] 48
    $rng.GetBytes($bytes)
    $secret = [Convert]::ToBase64String($bytes)

    # Read explicitly as UTF-8: PowerShell 5.1 defaults to ANSI and corrupts
    # any non-ASCII characters in the template.
    $example = [System.IO.File]::ReadAllText((Join-Path (Get-Location) ".env.example"), [System.Text.Encoding]::UTF8)
    $content = $example -replace "JWT_SECRET=.*", "JWT_SECRET=$secret"
    [System.IO.File]::WriteAllText(
        (Join-Path (Get-Location) ".env"),
        $content,
        (New-Object System.Text.UTF8Encoding $false)
    )
    Write-Output "created .env (with a generated JWT secret)"
} else {
    Write-Output ".env already exists"
}

if (-not (Test-Path infra/docker/.env)) {
    Copy-Item infra/docker/.env.example infra/docker/.env
    Write-Output "created infra/docker/.env"
} else {
    Write-Output "infra/docker/.env already exists"
}

if (-not (Test-Path frontend/.env.local)) {
    Copy-Item frontend/.env.example frontend/.env.local
    Write-Output "created frontend/.env.local"
} else {
    Write-Output "frontend/.env.local already exists"
}
