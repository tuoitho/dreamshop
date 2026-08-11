# Pull-request Workflow

## Contributor workflow

1. Create a short-lived feature branch from the current `main`.
2. Keep the change scoped to the owning service, package, or application surface.
3. Open a pull request into `main`.
4. Complete the impact and verification sections in the pull-request template.
5. Obtain at least one approval and satisfy every required status check.
6. Squash merge after review; do not merge directly to `main`.

The pull-request title is the final squash commit subject. Intermediate commits
are not required to use Conventional Commits.

## Administrator settings

Tracked documentation records the intended settings; it does not configure
GitHub remotely. Configure the `main` branch to:

- require a pull request before merging;
- require one approval and dismiss stale approvals;
- require conversation resolution;
- require the checks `PR title`, `Contracts`, `Java`, `Go`, and `Frontend`;
- require the branch to be up to date if the repository team adopts that setting consistently;
- block force pushes and branch deletion;
- enable squash merge and disable merge commits and rebase merges.
