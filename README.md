# go-cover-view

This was forked from [johejo/go-cover-view](https://github.com/johejo/go-cover-view) for a few reasons:

1. The original didn't have support for org repos
2. I think #2 is related to #1, but the event data didn't contain the PR number - or possibly this is because of just a different flow in general
3. Need support for multiple comments and the ability to view percentage (this is not implemented yet)
4. The ability to forward coverage information to a datadog metric

simple go coverage report viewer

You can see [the changelog here](https://github.com/hedenface/go-cover-view/blob/master/CHANGELOG.md).

## Install

```
go get github.com/hedenface/go-cover-view
go install github.com/hedenface/go-cover-view
```

## Basic Documentation

You can read the original documentation in the original repository's [README](https://github.com/johejo/go-cover-view/blob/master/README.md). Most of the functionality remains similar. The biggest difference is the way that the GitHub Actions Integration works. You need to pass the PR number to the comment step (currently I recommend using [8BitJonny/gh-get-current-pr@2.0.0](https://github.com/8BitJonny/gh-get-current-pr). Also, this currently sends a custom metric to a datadog endpoint. Future plans for decoupling this exist currently.

Ultimately, the functionality that exists in the gh-get-current-pr step will be duplicated inside this project.

## GitHub Actions Integration

GitHub Actions workflow example

```yaml
name: ci
on:
  pull_request:
    branches:
      - master
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest]
        go: ["1.14"]
    runs-on: ${{ matrix.os }}
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: ${{ matrix.go }}

      - name: "test"
        run: |
          go test -cover -coverprofile coverage.txt -race -v ./...

      - name: get PR number
        uses: 8BitJonny/gh-get-current-pr@2.0.0
        id: pr_number
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}

      - name: pull request coverage comment
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          PR_NUMBER: ${{ steps.pr_number.outputs.number }}
          DD_SITE: "datadoghq.com"
          DD_API_KEY: ${{ secrets.DATADOG_API_KEY }}
          PROJECT: "name-of-software"
          VERTICAL: "business-vertical"
          PROJECT_TYPE: "Backend" # can be Backend, Frontend, or Other
        run: |
          git fetch origin main
          go install github.com/hedenface/go-cover-view@2.0.1
          go-cover-view -git-diff-base origin/main
```

## License

MIT

## Author

* Mitsuo Heijo (@johejo)
* Bryan Heden (@hedenface)
