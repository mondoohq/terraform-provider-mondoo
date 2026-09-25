# Review checklist: terraform-plugin-framework v1.19 plus this repo's history

Each item is laid out as **rule**, then *diff tell*, then the real bug(s) it caused. `ip/` = `internal/provider/`.

## Schema and plan

- **The provider may only write a value the user didn't set into an attribute that is `Computed`.** Optional-without-Computed plus `data.X = <server value>` in Create/Read gives `inconsistent result after apply: .x was null, now "…"`.
  - *Tell:* `Optional: true` without `Computed` on `space_id`, `name`, `expiration`, `after_days` or anything the API defaults.
  - Bugs: PR 145 → 146 (space_id on ~12 resources), #70, PR 212, PR 268.
- **Optional+Computed stable values need `UseStateForUnknown`.** Without it, every plan shows "(known after apply)" noise. Don't add it to values that change on update.
- **`UseStateForUnknown` inside `ListNested`/`SetNested` keeps a *null* prior value since framework 1.15.1.** Adding new elements then gives "inconsistent result". Use `UseNonNullStateForUnknown`.
- **Optional+Computed objects:**
  - Every child also needs Optional+Computed. A `Required` child inside an optional object makes a partial config fail.
  - Every child needs `UseStateForUnknown`, or Update receives unknowns.
  - Bugs: PR 268 → 318/367.
- **An Optional+Computed attribute the user *did* set must come back exactly as configured.** Create must send it to the API and must not overwrite it with a server-generated value. Bug: #376 (`exception_id`).
- **Attributes the API can't update in place need `RequiresReplace`.** This covers `space_id`, `scope_mrn`, `org_id`, and every field on resources whose Update is a no-op. Otherwise apply "succeeds" and nothing changes.
  - Bugs: PR 205 → 366, PR 227 (scope_mrn).
  - Adding `RequiresReplace` to an **already released** attribute that used to update in place is itself breaking: it destroys user objects.
- **Validators:**
  - Use `stringvalidator.OneOf` for enums, and check the values against `mondoovX` enum constants (PR 251: API renamed an enum).
  - Test every regex against a real ID; the error text must match the regex (PRs 264/281).
  - Use `ConflictsWith`/`ExactlyOneOf` rather than hand-written checks. `ExactlyOneOf` on Optional attributes conflicts with the provider-level space fallback (#298).
  - An explicit `false` or `""` counts as *configured* for `ExactlyOneOf`/`ConflictsWith`. `use_x = false` next to the alternative fails validation, while `use_x = false` alone passes plan and fails at apply. When the check depends on the value, do it in `ValidateConfig`.

## ValidateConfig / ModifyPlan

- **Optional nested blocks are nil pointers when omitted.** `data.A.B.C` needs a nil check at every level. Bug: PR 224 → 234 panicked `plan` for every config without `vpc_configuration`.
- **Values can be unknown at validate/plan time** (references to other resources, `for_each`). `.ValueString()` on an unknown returns `""`, which falsely fails "must not be empty" checks. Guard with `IsUnknown() || IsNull()`.
- **ModifyPlan:** `req.State.Raw.IsNull()` means create; `req.Plan.Raw.IsNull()` means destroy.

## Model struct and conversions

- **An Optional nested attribute must map to a pointer field (`*FooInput`).** A value-type struct fails with `Value Conversion Error … Received null value` on every config that omits it, including existing state on refresh. Bug: PR 257 → 259.
- **`.String()` on a framework value returns a Go-quoted string** (`"\"abc\""`). API inputs must use `.ValueString()`.
  - Bugs: PR 212 (Delete sent a quoted MRN), #427 (mappings, still open).
- **Never discard diagnostics.** `x, _ := types.ListValueFrom/MapValueFrom/ElementsAs/.As(...)` hides failures and leaves the value Unknown, which gives `invalid result object after apply … unknown value`.
  - Bugs: #427, PR 358.
  - `MapValueFrom` needs a `map[string]T`, not a `[]KeyValue`.
- **`ConvertSlice[mondoov1.Int]` panics** (int32 → graphql.Int). Use `ConvertSliceInt32` (PR 209).
- **`obj.As(ctx, &t, basetypes.ObjectAsOptions{})` errors on unknown children.** Use `UnhandledUnknownAsEmpty`/`UnhandledNullAsEmpty` deliberately (PR 268 → 318).
- **Null vs empty:**
  - An omitted attribute must stay null, not `""`/`[]`/`{}`, and the reverse: an empty API list for a configured `[]` must come back as `[]`.
  - Flatteners that return a non-nil empty struct for "no settings" produce a diff.
- **Copy-paste field mixups** (`Subject: data.SpaceID`, PR 205 → 211; `SpaceMrn` vs `ScopeMrn`, PR 265). For every field in each API input struct, check that its source attribute has the same name.

## Create

- **`resp.State.Set` must run whenever the remote object exists.** Watch for `AddWarning(...); return` or an `AddError` after a successful mutation, placed before `State.Set`. The object is orphaned and the next apply makes a duplicate.
  - Bugs: #185, #282, PR 194 → 304.
  - `TriggerAction` failure is a warning **without** return (see `integration_crowdstrike_resource.go`).
- **Store IDs in the form the user configured.** `space_id` holds the **ID**: `SpaceFrom(mrn).ID()`, never `OwnerMrn`/scope MRN. Bug: PR 209 → 214, and the Read in PR 205.
- **Fill every Computed attribute from the API response, not from plan guesses.** If the response lacks a field (e.g. tags, PR 399), keep the planned value explicitly.

## Read

- **New resources must actually call the API.** 25 existing resources have a `// Read API call logic` no-op Read (#404). Don't let new ones copy it: drift is never detected and out-of-band deletes are invisible.
- **Every schema attribute must be set in Read.** Missing attributes cause diffs and break import. Bugs: #105 (roles never read), `mappings` always null in PR 205.
- **Not-found must call `resp.State.RemoveResource(ctx)` and return with no error.**
  - Check with `isNotFoundError(err)` (`ip/asset_routing_common.go`); see `integration_azure_resource.go` for the pattern.
  - Every other error must still be an error. Don't remove on any error (`integration_domain_resource.go` does this wrong).
- **Normalize what the API returns** (MRN ↔ ID, role short names vs MRNs: `RoleListNormalizerModifier`/`customtypes.RoleValue`, ordering) or the diff never goes away. Use a Set, or semantic equality, for unordered API lists (PR 378).

## Update

- **Send every mutable attribute.** Check the schema against the API input struct field by field. Bug: #120 (service_account roles never sent).
- **Derive scope/space exactly as Create does.** Better, share one helper. Bug: PR 227 → 266 (Update ignored `scope_mrn`).
- **Set the whole state** with `resp.State.Set(ctx, &data)`, not `SetAttribute` for one field. No unknowns may remain.
- **Switching modes on an existing object** (cert ↔ WIF, `use_x` flips, one nested credential block swapped for another):
  - Computed values that depend on the mode go stale under `UseStateForUnknown`. Mark them unknown in ModifyPlan when the driver attribute changes, then re-read them in Update.
  - When switching back, send an explicit `false` rather than omitting the flag, because the API may treat an omitted field as "unchanged".

## Delete

- **Use `.ValueString()` for MRNs.** Treat not-found as success. A parent that is already gone (space deleted) must not block destroy forever (#373).

## Import

- **Missing `ImportState` is the most common reviewer blocker here** (#167–171, #227). Integrations use `r.client.ImportIntegration`.
- **After import + Read, every attribute is set.** Secrets are null and documented. Check the diagnostics from `resp.State.Set`, since 29 existing call sites drop them.
- **Add `examples/resources/<name>/import.sh`.**

## GraphQL (`ip/gql.go`)

mondoo-go ships input types only, not the output schema, so query structs can't be verified offline. Flag these under *Needs verification* and ask for an acceptance test with an import step:

- **Inline fragments must name the *output* type** (`... on SentinelOneConfigurationOptions`), not an `*Input` type or a sibling's type.
  - `ClientIntegrationConfigurationOptions` is shared by **every** integration import, so one bad fragment breaks all of them (PR 187, #274, PR 194).
- **Field casing must match** (`memberCID`).
- **Payload field types must match** (PR 323: `errors` typed String, backend Map).
- **Mutation variables must be complete** (PR 271: missing `$after`).

## Breaking changes (compare with the latest release tag)

- **Removing, renaming or retyping a released attribute breaks users,** as does Optional→Required, changing a `Default`, or list↔set. The provider has no `UpgradeState` anywhere.
  - Required path: keep the old attribute with `DeprecationMessage` (naming the replacement), read/write both during transition (PR 399 did this for `tags`), and remove it only in a major version.
  - Bug: PR 213 (`space_id` → `scope_mrn` after v0.22.0 shipped it).
- **Data source inputs turned Computed-only** break `data` blocks with "Invalid Configuration for Read-Only Attribute" (PR 145, space data source `mrn`).
- **Changing `space_id` precedence or scope priority** (space vs org) silently moves where objects are created (PR 145, service_account).

## Contracts outside the schema: the customer's own HCL

Users copy our examples into pipelines and wire our outputs into *other* providers' resources (`aws_cloudformation_stack`, `azuread_*`, `google_*`). Those configs break just as surely as a schema change does, and they aren't pinned to our provider version.

- **An example gains a required input for a downstream resource** (a new `parameters`/argument key, a renamed variable, a retired template). Every existing user config without it breaks on its next apply, including configs written against older provider versions.
  - This is a **Blocker** unless the PR (or its release notes) gives the migration: what to add, and which versions or stacks are affected.
  - Bug: PR 441. The serverless template made `MondooSourceBucket` required (serverless-scanner-aws #790), the example quietly gained it, and a customer pipeline broke with no notice.
- **A computed attribute that points to an external artifact** (template URL, script, bucket, image):
  - Check whether the URL is versioned. An unversioned "latest" URL (`mondoo-serverless-v2.json`) means any upstream change reaches every user with no provider release and no changelog. Flag this under *Needs verification* and ask for a versioned URL or a documented compatibility policy.
  - When the value is derived from another attribute (e.g. region → bucket), apply the Update "switching modes" rule. `UseStateForUnknown` plus a server recompute gives "inconsistent result after apply" (PR 441, `region`).
- **The PR says an upstream (server, template, API) now requires or rejects something.** Ask whether that upstream change is already live. If it is, users are broken *before* this provider version ships, and the fix belongs in the release notes and customer comms, not just in the example.

## Secrets

- **Credential fields need `Sensitive: true`.** For generated integrations, `gen/gen.go` decides this through `isSensitiveField`.
- **Adding `Sensitive` to a released attribute** doesn't touch state. It does make users' `output` blocks that reference the attribute fail with `Output refers to sensitive values` until they add `sensitive = true`. Put it in the Release notes section.
- **Never log inputs containing secrets.** `tflog.Debug(... fmt.Sprintf("%+v", input))` is common here, so check what the input holds.

## House rules (CI enforces some; reviewers ask for all)

- **New hand-written resources are registered in `Resources()`/`DataSources()` in `ip/provider.go`.** Generated code: `gql_generated.go`, `provider_generated.go`, and the integrations listed in `gen/gen.go`'s `ClientIntegrationConfigurationInput{...}` (currently okta, google_workspace, azure_devops). Changes go in `gen/templates/`; `go run gen/gen.go` must reproduce the committed files.
- **After a schema or description change, run `make generate` and commit `docs/`.** Never hand-edit `docs/`.
- **Examples:** `examples/resources/mondoo_<name>/{main.tf,resource.tf,import.sh}`. Examples must pass `terraform fmt` and tflint.
  - Usage comments must name variables that are actually declared.
  - Two files in one directory must not both declare the same `provider`/`variable`.
- **New files carry the BUSL copyright header** (the license job checks it).
- **Tests:** acceptance test with Create, an Update step, and `ImportState: true, ImportStateVerify: true`. Model it on `integration_shodan_resource_test.go`. Tests that would have caught bugs above:
  - `ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}` for drift and normalization
  - a step that omits optional blocks, for nil/null bugs
  - `plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)` to prove an update doesn't replace
  - unit tests that build the model struct directly can't catch null-conversion bugs; go through `tfsdk.Config`/acceptance
