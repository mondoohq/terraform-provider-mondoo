---
name: review-provider-pr
description: Use when reviewing a pull request, branch, or diff in terraform-provider-mondoo, or when asked whether a provider change is safe to merge or release
---

# Review a terraform-provider-mondoo PR

## Overview

Find what will break **users' Terraform runs**: failed plans, "Provider produced inconsistent result after apply", perpetual diffs, orphaned or duplicated remote objects, panics, broken imports, and broken upgrades of existing state. Style nits are not the job.

Every bug listed in `checklist.md` shipped past human review in this repo. Walk each changed resource through its whole lifecycle, don't just read the hunks.

## Process

1. **Get the change.**
   - PR number: `gh pr view N --json title,body,headRefName,files` and `gh pr diff N`.
   - Check it out outside the repo so you can read whole files: `git fetch origin pull/N/head && git worktree add --detach "$(mktemp -d)/pr-N" FETCH_HEAD`. Remove it when done with `git worktree remove`.
   - `gh pr checks N`: CI acceptance tests (TF_ACC, TF 1.11–1.13) run against the **real API**. A green run is evidence that GraphQL query shapes are accepted. The `generate` job proves docs are regenerated. Read logs early with `gh run view --log`, because they expire.
   - Branch or working tree: diff against `origin/main`.
   - Local tests need credentials: `TestMain` in `internal/provider` creates a real space and panics without them. Use **only** `TERRAFORM_TESTING_MONDOO_BASE64`, a service account for the `mondoo-terraform-testing` org (see README → Test). Never use an existing `MONDOO_CONFIG_*` or the CLI config, which may point at a production org. If the variable isn't set, rely on CI.
2. **Find what's already released.**
   - `git fetch --tags`, then `LAST=$(git describe --tags --abbrev=0 --match 'v*' HEAD)`.
   - For every schema attribute the PR removes, renames, retypes or re-requires: `git show $LAST:<file> | grep '"attr"'`.
   - If the attribute exists in `$LAST`, the change breaks users. If it was added after `$LAST`, it's free to change.
   - Run `git diff $LAST -- examples/`. Users copy these configs into their pipelines. A new required argument or parameter, a removed variable, or a changed template breaks every existing copy (see "Contracts outside the schema" in `checklist.md`).
3. **Walk the lifecycle** for every resource or data source the diff touches. Read the whole file and every helper it calls in `gql.go` / `conversions.go`, then trace, in order:
   - ValidateConfig and plan (values unknown, blocks omitted)
   - Create
   - Read (refresh, and deleted out-of-band)
   - Update (each mutable attribute)
   - Delete
   - Import
   - Upgrade from `$LAST` state

   At each step, apply `checklist.md`.

   If the PR changes no resource behaviour (docs, examples, CI, schema flags such as `Sensitive`), skip this walk. Instead check what users will see (release notes) and whether a new guard or test really covers what it claims to.
4. **Prove each finding.** Name the config or state that triggers it and the exact Terraform error or behaviour it causes. If you can't point to the line that goes wrong, put it under *Needs verification*, not in Blockers.
5. **Reproduce blockers and needs-verification items** when `TERRAFORM_TESTING_MONDOO_BASE64` is set:
   - Write the test that catches it as a new `*_repro_test.go` in the PR worktree. Model it on an existing `TestAcc…`: use `accSpace.ID()`, and add a plancheck or a follow-up step.
   - Run it with `env -u MONDOO_API_ENDPOINT -u MONDOO_CONFIG_PATH -u MONDOO_API_TOKEN MONDOO_CONFIG_BASE64="$TERRAFORM_TESTING_MONDOO_BASE64" TF_ACC=1 go test ./internal/provider/ -run <TestName> -v -timeout 5m`. The `-u` flags matter. `MONDOO_API_ENDPOINT` overrides the service account's endpoint in the provider (not in `TestMain`), so a stray export would send the testing credentials to another environment.
   - If the variable isn't visible in your shell (it's a snapshot from session start), ask the user to run that exact command themselves. In Claude Code, they can type it prefixed with `!` so the output lands in the conversation. Do not go looking for the value.
   - Reproduced: quote the Terraform error in the finding. Didn't reproduce: move it down a section or drop it.
   - Never push a repro test to someone else's branch unless asked. The test goes in the finding instead.
6. **Separate pre-existing problems.** A bug in code the PR didn't touch or copy is one line under *Pre-existing*. A bug the PR copied into new code (e.g. a no-op Read in a new resource) counts as introduced, even when a sibling resource has the same bug. Mention the sibling under *Pre-existing*.

## Output

```
## Verdict: BLOCK | FIX BEFORE MERGE | OK TO MERGE

### Blockers   (breaks users: failed plan/apply, lost/orphaned objects, panic, breaking upgrade, existing user HCL that stops working)
1. file:line — what's wrong
   Triggers when: <config/state>
   User sees: <exact error or behaviour>
   Fix: <one line>
   Test that catches it: <TestStep / plancheck / ImportStateVerify>
   Reproduced: yes (<error excerpt>) | not run (<why>)

### Should fix   (drift, missing import/tests/docs, unvalidated input)
### Needs verification   (depends on API behaviour you can't see offline)
### Release notes   (user-visible behaviour changes to call out, e.g. new RequiresReplace, Sensitive on a released attribute)
### Pre-existing   (one line each, not caused by this PR)
```

The verdict is **BLOCK** if there are any Blockers. It is **FIX BEFORE MERGE** if a Should-fix or Needs-verification item hits users of the feature this PR adds. Otherwise it is **OK TO MERGE**.

Leave out empty sections. No praise and no summary of what the PR does. Don't post to GitHub unless asked. If asked, use `gh pr review N --comment --body-file`.

## Scale

For diffs over ~1500 lines, dispatch one subagent per changed resource, all in parallel (they're read-only, so they can share one worktree). Give each one this skill's checklist and the `$LAST` tag, then merge and dedupe the findings yourself.
