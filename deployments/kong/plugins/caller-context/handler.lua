local cjson = require "cjson"
local pkey = require "resty.openssl.pkey"
local resty_http = require "resty.http"

local CallerContextHandler = {
  VERSION = "1.0.0",
  PRIORITY = 1001,
}

local CACHE_KEY_PREFIX = "caller-context:jwks:"

local function b64url_decode(value)
  local b64 = value:gsub("-", "+"):gsub("_", "/")
  local pad = (4 - #b64 % 4) % 4
  return ngx.decode_base64(b64 .. string.rep("=", pad))
end

local function decode_json_part(part)
  local raw = b64url_decode(part)
  if not raw then
    return nil
  end
  local ok, value = pcall(cjson.decode, raw)
  if not ok or type(value) ~= "table" then
    return nil
  end
  return value
end

local function aud_contains(aud, expected)
  if type(aud) == "string" then
    return aud == expected
  end
  if type(aud) == "table" then
    for _, item in ipairs(aud) do
      if item == expected then
        return true
      end
    end
  end
  return false
end

local function verify_claims(claims, conf)
  if claims.iss ~= conf.issuer then
    return false, "issuer mismatch"
  end
  if not aud_contains(claims.aud, conf.audience) then
    return false, "audience mismatch"
  end
  local now = ngx.time()
  if type(claims.exp) ~= "number" or now > claims.exp + conf.clock_skew_seconds then
    return false, "token expired"
  end
  return true
end

local function fetch_jwks(jwks_uri)
  local httpc = resty_http.new()
  httpc:set_timeout(2000)
  local res, err = httpc:request_uri(jwks_uri, { method = "GET" })
  if not res then
    return nil, err
  end
  if res.status ~= 200 then
    return nil, "JWKS endpoint returned " .. res.status
  end
  local ok, decoded = pcall(cjson.decode, res.body)
  if not ok or type(decoded) ~= "table" or type(decoded.keys) ~= "table" then
    return nil, "invalid JWKS body"
  end
  return decoded
end

local function load_jwks(jwks_uri)
  local jwks, err = fetch_jwks(jwks_uri)
  if not jwks then
    kong.log.warn("caller-context: JWKS fetch failed: ", err)
    return false
  end
  return jwks
end

local function find_key(jwks, kid)
  for _, key in ipairs(jwks.keys) do
    if key.kid == kid then
      return key
    end
  end
end

local function verify_signature(header, header_part, payload_part, signature_part, jwk)
  if header.alg ~= "RS256" or type(jwk.n) ~= "string" or type(jwk.e) ~= "string" then
    return false
  end
  local pk, err = pkey.new(cjson.encode({ kty = "RSA", n = jwk.n, e = jwk.e }), { format = "JWK" })
  if not pk then
    kong.log.warn("caller-context: failed to load JWK: ", err)
    return false
  end
  local signature = b64url_decode(signature_part)
  if not signature then
    return false
  end
  return pk:verify(signature, header_part .. "." .. payload_part, "sha256")
end

function CallerContextHandler:access(conf)
  kong.service.request.clear_header(conf.user_id_header)
  kong.service.request.clear_header(conf.roles_header)

  local auth_header = kong.request.get_header("Authorization")
  if type(auth_header) ~= "string" then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end
  local token = auth_header:match("^[Bb]earer%s+(%S+)$")
  if not token then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end

  local header_part, payload_part, signature_part = token:match("^([^.]+)%.([^.]+)%.([^.]+)$")
  if not header_part or not payload_part or not signature_part then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Missing or invalid access token" })
  end

  local header = decode_json_part(header_part)
  local claims = decode_json_part(payload_part)
  if not header or not claims or not header.kid then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  local cache_key = CACHE_KEY_PREFIX .. conf.jwks_uri
  local jwks = kong.cache:get(cache_key, { ttl = conf.cache_ttl_seconds }, load_jwks, conf.jwks_uri)
  if not jwks then
    return kong.response.exit(503, { code = "AUTHENTICATION_UNAVAILABLE", message = "Authentication is temporarily unavailable" })
  end

  local jwk = find_key(jwks, header.kid)
  if not jwk then
    kong.cache:invalidate(cache_key)
    jwks = load_jwks(conf.jwks_uri)
    if not jwks then
      return kong.response.exit(503, { code = "AUTHENTICATION_UNAVAILABLE", message = "Authentication is temporarily unavailable" })
    end
    jwk = find_key(jwks, header.kid)
  end
  if not jwk then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  local claims_ok, claims_err = verify_claims(claims, conf)
  if not claims_ok then
    kong.log.debug("caller-context: claims rejected: ", claims_err)
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end
  if not verify_signature(header, header_part, payload_part, signature_part, jwk) then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end
  if type(claims.sub) ~= "string" or claims.sub == "" then
    return kong.response.exit(401, { code = "UNAUTHORIZED", message = "Invalid or expired access token" })
  end

  kong.service.request.set_header(conf.user_id_header, claims.sub)
  if type(claims.roles) == "table" then
    kong.service.request.set_header(conf.roles_header, table.concat(claims.roles, ","))
  end
end

return CallerContextHandler
