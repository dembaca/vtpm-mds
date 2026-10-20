## ADDED Requirements

### Requirement: Local Lab Builds Pass lintian Whatever The Branch

A local lab build (`make deb-local`) SHALL stamp the changelog with an entry
that names the commit, the branch and whether the working tree was dirty, and
every line of that entry SHALL be at most 80 columns long, whatever the branch
is called, so that `lintian --fail-on error,warning` does not fail on the
stamp. A branch name too long for its line SHALL be shortened; the commit
SHALL never be shortened beyond the 12-character abbreviation the version
carries.

#### Scenario: Long branch name

- **GIVEN** a checkout on a branch whose name is 90 characters long
- **WHEN** `make deb-local` builds both packages
- **THEN** no line of the stamped changelog entry exceeds 80 columns, and
  `lintian --fail-on error,warning` reports no `debian-changelog-line-too-long`

#### Scenario: The build is still identifiable

- **GIVEN** any checkout
- **WHEN** `make deb-local` builds the host package
- **THEN** its version is `<base>+git<date>.<12-character sha>`, with `.dirty`
  when the tree had changes, and the changelog entry names the same commit
