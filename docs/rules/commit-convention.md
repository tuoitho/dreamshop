# Pull-request Title Convention

The pull-request title becomes the final squash commit subject. Intermediate
branch commits do not need to follow this convention.

## Grammar

```text
<type>(<optional-scope>)<optional-breaking-marker>: <description>
```

Allowed types are `feat`, `fix`, `docs`, `refactor`, `test`, `build`,
`ci`, `chore`, `perf`, and `revert`. An optional scope uses lowercase
letters, digits, and hyphens. The description must be non-empty.

Validate locally:

```powershell
pwsh -NoProfile -File scripts/validate-pr-title.ps1 -Title 'feat(order): add checkout validation'
```

Valid examples:

- `feat(order): add checkout validation`
- `fix!: reject reused refresh tokens`
- `docs: clarify local startup`

Invalid examples:

- `Feat(order): add checkout validation` (uppercase type)
- `feat(order_api): add checkout validation` (underscore in scope)
- `feat(order) add checkout validation` (missing colon-space)
- `feat(order):` (empty description)

