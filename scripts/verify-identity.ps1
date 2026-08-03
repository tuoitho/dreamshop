[CmdletBinding()]
param(
    [string] $BaseUrl = 'http://localhost:8000/api/v1/auth',
    [string] $ExpectedIssuer = 'http://identity-service:8080'
)

$ErrorActionPreference = 'Stop'

function ConvertFrom-Base64Url([string] $Value) {
    $padded = $Value.Replace('-', '+').Replace('_', '/')
    switch ($padded.Length % 4) {
        2 { $padded += '==' }
        3 { $padded += '=' }
        1 { throw 'Invalid Base64URL value.' }
    }
    return [Convert]::FromBase64String($padded)
}

function Assert-StatusCode([object] $Response, [int] $Expected, [string] $Operation) {
    if ($Response.StatusCode -ne $Expected) {
        throw "$Operation returned unexpected HTTP status $($Response.StatusCode)."
    }
}

$base = $BaseUrl.TrimEnd('/')
$email = "identity-smoke-$([Guid]::NewGuid().ToString('N'))@example.test"
$password = "local-only-$([Guid]::NewGuid().ToString('N'))"
$requestBody = @{ email = $email; password = $password; full_name = 'Identity Smoke Test' } | ConvertTo-Json -Compress

$register = Invoke-WebRequest -Uri "$base/register" -Method Post -ContentType 'application/json' -Body $requestBody -SkipHttpErrorCheck
Assert-StatusCode $register 201 'Registration'

$loginBody = @{ email = $email; password = $password } | ConvertTo-Json -Compress
$login = Invoke-WebRequest -Uri "$base/login" -Method Post -ContentType 'application/json' -Body $loginBody -SkipHttpErrorCheck
Assert-StatusCode $login 200 'Login'

$accessToken = ($login.Content | ConvertFrom-Json).access_token
if ([string]::IsNullOrWhiteSpace($accessToken)) {
    throw 'Login response did not contain an access token.'
}

$tokenParts = $accessToken.Split('.')
if ($tokenParts.Count -ne 3) {
    throw 'Access token is not a compact JWT.'
}

$header = [Text.Encoding]::UTF8.GetString((ConvertFrom-Base64Url $tokenParts[0])) | ConvertFrom-Json
$claims = [Text.Encoding]::UTF8.GetString((ConvertFrom-Base64Url $tokenParts[1])) | ConvertFrom-Json
if ($header.alg -ne 'RS256' -or [string]::IsNullOrWhiteSpace($header.kid)) {
    throw 'Access token does not declare RS256 and a key identifier.'
}

$jwks = (Invoke-WebRequest -Uri "$base/.well-known/jwks.json" -SkipHttpErrorCheck)
Assert-StatusCode $jwks 200 'JWKS retrieval'
$jwk = ((($jwks.Content | ConvertFrom-Json).keys) | Where-Object { $_.kid -eq $header.kid } | Select-Object -First 1)
if ($null -eq $jwk -or $jwk.kty -ne 'RSA' -or [string]::IsNullOrWhiteSpace($jwk.n) -or [string]::IsNullOrWhiteSpace($jwk.e)) {
    throw 'JWKS does not contain the RSA key referenced by the access token.'
}

$rsaParameters = [System.Security.Cryptography.RSAParameters]@{
    Modulus = ConvertFrom-Base64Url $jwk.n
    Exponent = ConvertFrom-Base64Url $jwk.e
}
$rsa = [System.Security.Cryptography.RSA]::Create()
try {
    $rsa.ImportParameters($rsaParameters)
    $signedData = [Text.Encoding]::ASCII.GetBytes("$($tokenParts[0]).$($tokenParts[1])")
    $signature = ConvertFrom-Base64Url $tokenParts[2]
    if (-not $rsa.VerifyData($signedData, $signature, [System.Security.Cryptography.HashAlgorithmName]::SHA256, [System.Security.Cryptography.RSASignaturePadding]::Pkcs1)) {
        throw 'Access token signature verification failed.'
    }
} finally {
    $rsa.Dispose()
}

if ([string]::IsNullOrWhiteSpace($claims.sub)) {
    throw 'Access token subject is missing.'
}
if (@($claims.roles) -notcontains 'CUSTOMER') {
    throw 'Access token does not contain the CUSTOMER role.'
}
if ($claims.iss -ne $ExpectedIssuer) {
    throw 'Access token issuer does not match the expected issuer.'
}
if (@($claims.aud) -notcontains 'ecommerce-api') {
    throw 'Access token audience does not contain ecommerce-api.'
}
if ($null -eq $claims.exp -or [DateTimeOffset]::FromUnixTimeSeconds([long] $claims.exp) -le [DateTimeOffset]::UtcNow) {
    throw 'Access token expiry is missing or not in the future.'
}

$duplicate = Invoke-WebRequest -Uri "$base/register" -Method Post -ContentType 'application/json' -Body $requestBody -SkipHttpErrorCheck
Assert-StatusCode $duplicate 409 'Duplicate registration'
if (($duplicate.Content | ConvertFrom-Json).code -ne 'EMAIL_ALREADY_REGISTERED') {
    throw 'Duplicate registration did not return EMAIL_ALREADY_REGISTERED.'
}

Write-Host 'Identity register/login/JWKS smoke checks passed.'
