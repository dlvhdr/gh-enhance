Get latest checks of a PR (contains multiple runs with jobs within each):

```sh
gh pr checks --json name,workflow,link 34285 -R neovim/neovim > ./examples/pr-34285-checks.json`
```

Get all jobs of a run:

```sh
gh run view 15468370618 -R neovim/neovim --json name,jobs > ./examples/pr-34285-run-15468370618-jobs.json
```

Get a run's logs:

```sh
gh run view 15468370618 -R neovim/neovim --log > ./examples/pr-34285-run-15468370618-jobs-logs.json
```

Get a job's logs:

```sh
gh api \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      /repos/neovim/neovim/actions/jobs/43545960562V/
```
