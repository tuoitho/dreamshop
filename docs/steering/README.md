# Steering Documentation

The repository uses progressive disclosure: load the smallest instruction set
that fully governs the change.

## Reading Order

1. Read the root [`AGENTS.md`](../../AGENTS.md).
2. Read the nearest local `AGENTS.md` for each service or frontend surface in
   scope, when one exists.
3. Use the routing table below to select only the relevant topic documents.
4. Read linked ADRs when the change challenges or extends an accepted decision.

Local instructions specialize repository rules for their directory. They do not
override repository-wide service boundaries, compatibility, security, or
verification requirements. ADRs record architectural decisions; these steering
documents define how changes must comply with them.

## Routing Table

| Change type | Required steering |
| --- | --- |
| Ownership, shared code, persistence ownership, or service interaction | [`architecture.md`](architecture.md), [`service-boundaries.md`](service-boundaries.md), and [`change-impact.md`](change-impact.md) |
| REST, OpenAPI, protobuf, gRPC, event, error, or idempotency contract | [`api-contracts.md`](api-contracts.md), [`change-impact.md`](change-impact.md), and [`testing.md`](testing.md) |
| Work spanning multiple services, frontend/backend, contracts, or infrastructure | [`change-impact.md`](change-impact.md), plus the topic documents for every affected category |
| Tests, regression fixes, contract checks, or end-to-end verification | [`testing.md`](testing.md) and, when external resources are involved, [`delivery.md`](delivery.md) |
| Database migration, environment variable, secret, Kong route, manifest, deployment, or rollout | [`delivery.md`](delivery.md), [`change-impact.md`](change-impact.md), and any applicable contract/boundary document |

## Minimal Reading Examples

- A domain-only calculation inside Order: root instructions, Order's local
  instructions, and [`testing.md`](testing.md).
- A new protobuf field consumed by BFF: root instructions, both local instruction
  files, [`api-contracts.md`](api-contracts.md),
  [`change-impact.md`](change-impact.md), and [`testing.md`](testing.md).
- A Flyway migration inside Identity: root instructions, Identity's local
  instructions, [`delivery.md`](delivery.md), and [`testing.md`](testing.md).

