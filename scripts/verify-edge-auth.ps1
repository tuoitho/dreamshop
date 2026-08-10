[CmdletBinding()]
param(
    [string] $AuthBaseUrl = 'http://localhost:8000/api/v1/auth',
    [string] $UsersBaseUrl = 'http://localhost:8000/api/v1/users'
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

$authBase = $AuthBaseUrl.TrimEnd('/')
$usersBase = $UsersBaseUrl.TrimEnd('/')
$email = "edge-auth-$([Guid]::NewGuid().ToString('N'))@example.test"
$password = "local-only-$([Guid]::NewGuid().ToString('N'))"
$requestBody = @{ email = $email; password = $password; full_name = 'Edge Auth Smoke Test' } | ConvertTo-Json -Compress

$register = Invoke-WebRequest -Uri "$authBase/register" -Method Post -ContentType 'application/json' -Body $requestBody -SkipHttpErrorCheck
Assert-StatusCode $register 201 'Registration'

$login = Invoke-WebRequest -Uri "$authBase/login" -Method Post -ContentType 'application/json' -Body (@{ email = $email; password = $password } | ConvertTo-Json -Compress) -SessionVariable authSession -SkipHttpErrorCheck
Assert-StatusCode $login 200 'Login'

$accessToken = ($login.Content | ConvertFrom-Json).access_token
if ([string]::IsNullOrWhiteSpace($accessToken)) {
    throw 'Login response did not contain an access token.'
}

$claims = [Text.Encoding]::UTF8.GetString((ConvertFrom-Base64Url $accessToken.Split('.')[1])) | ConvertFrom-Json

$refresh = Invoke-WebRequest -Uri "$authBase/refresh" -Method Post -WebSession $authSession -SkipHttpErrorCheck
Assert-StatusCode $refresh 200 'Refresh'
$accessToken = ($refresh.Content | ConvertFrom-Json).access_token
if ([string]::IsNullOrWhiteSpace($accessToken)) {
    throw 'Refresh response did not contain an access token.'
}
$latestCookie = $authSession.Cookies.GetCookies([Uri] $AuthBaseUrl) |
    Where-Object { $_.Name -eq 'refresh_token' } |
    Select-Object -First 1
if ($null -eq $latestCookie -or [string]::IsNullOrWhiteSpace($latestCookie.Value)) {
    throw 'Refresh did not rotate the refresh cookie.'
}
$latestCookieValue = $latestCookie.Value

$me = Invoke-WebRequest -Uri "$usersBase/me" -Headers @{ Authorization = "Bearer $accessToken" } -SkipHttpErrorCheck
Assert-StatusCode $me 200 'Authenticated /users/me'
$meBody = $me.Content | ConvertFrom-Json
if ($meBody.user_id -ne $claims.sub) {
    throw '/users/me did not return the caller matching the token subject.'
}
if (@($meBody.roles) -notcontains 'CUSTOMER') {
    throw '/users/me did not return the CUSTOMER role.'
}
if ($meBody.email -ne $email) {
    throw '/users/me did not return the registered email.'
}

$updateBody = @{ full_name = 'Updated Edge Auth Smoke Test' } | ConvertTo-Json -Compress
$updated = Invoke-WebRequest -Uri "$usersBase/me" -Method Patch -Headers @{ Authorization = "Bearer $accessToken" } -ContentType 'application/json' -Body $updateBody -SkipHttpErrorCheck
Assert-StatusCode $updated 200 'PATCH /users/me'
if (($updated.Content | ConvertFrom-Json).full_name -ne 'Updated Edge Auth Smoke Test') {
    throw 'PATCH /users/me did not return the updated full name.'
}

$anonymous = Invoke-WebRequest -Uri "$usersBase/me" -SkipHttpErrorCheck
Assert-StatusCode $anonymous 401 'Unauthenticated /users/me'
if (($anonymous.Content | ConvertFrom-Json).code -ne 'UNAUTHORIZED') {
    throw 'Unauthenticated /users/me did not return UNAUTHORIZED.'
}

$tokenParts = $accessToken.Split('.')
if ($tokenParts.Count -ne 3) {
    throw 'Access token is not a three-part JWT.'
}
$tamperedSignature = $tokenParts[2].Substring(0, $tokenParts[2].Length - 1) + $(if ($tokenParts[2].EndsWith('x')) { 'y' } else { 'x' })
$tamperedToken = "$($tokenParts[0]).$($tokenParts[1]).$tamperedSignature"
$tampered = Invoke-WebRequest -Uri "$usersBase/me" -Headers @{ Authorization = "Bearer $tamperedToken" } -SkipHttpErrorCheck
Assert-StatusCode $tampered 401 'Tampered-token /users/me'
if (($tampered.Content | ConvertFrom-Json).code -ne 'UNAUTHORIZED') {
    throw 'Kong accepted a tampered token signature.'
}

$forgedHeaders = @{ Authorization = "Bearer $accessToken"; 'X-User-Id' = [Guid]::NewGuid().ToString() }
$forged = Invoke-WebRequest -Uri "$usersBase/me" -Headers $forgedHeaders -SkipHttpErrorCheck
Assert-StatusCode $forged 200 'Forged-header /users/me'
if (($forged.Content | ConvertFrom-Json).user_id -ne $claims.sub) {
    throw 'Kong forwarded a client-supplied X-User-Id instead of the verified subject.'
}

$logout = Invoke-WebRequest -Uri "$authBase/logout" -Method Post -WebSession $authSession -SkipHttpErrorCheck
Assert-StatusCode $logout 204 'Logout'

$afterLogout = Invoke-WebRequest -Uri "$authBase/refresh" -Method Post -Headers @{ Cookie = "refresh_token=$latestCookieValue" } -SkipHttpErrorCheck
Assert-StatusCode $afterLogout 401 'Refresh after logout'
if (($afterLogout.Content | ConvertFrom-Json).code -ne 'INVALID_REFRESH_TOKEN') {
    throw 'Refresh after logout did not return INVALID_REFRESH_TOKEN.'
}

Write-Host 'Kong edge auth refresh/profile/logout smoke checks passed.'
