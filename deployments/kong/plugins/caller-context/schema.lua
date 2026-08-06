return {
  name = "caller-context",
  fields = {
    { config = {
        type = "record",
        fields = {
          { jwks_uri = { type = "string", required = true } },
          { issuer = { type = "string", required = true } },
          { audience = { type = "string", required = true } },
          { user_id_header = { type = "string", default = "x-user-id" } },
          { roles_header = { type = "string", default = "x-user-roles" } },
          { cache_ttl_seconds = { type = "integer", default = 300 } },
          { clock_skew_seconds = { type = "integer", default = 0 } },
        },
      } },
    },
  }
