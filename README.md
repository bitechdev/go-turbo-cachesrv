# go-turbo-cachesrv

Go Turbo Build Cache Server

## Enviroment Variables

TURBO_CACHE_DIR=
TURBO_AUTH_TOKEN=
TURBO_LOG_FILE=

## Usage

```
pnpm turbo login
pnpm turbo link

pnpm turbo run build --api="http://localhost:8080" --token="test"
```

## Setup a vercel account and get your tokens

    TURBO_TOKEN - The Bearer token to access the Remote Cache
    TURBO_TEAM - The account to which the monorepo belongs

Set these tokens into github actions.

```
# ...

jobs:
  build:
    name: Build and Test
    timeout-minutes: 15
    runs-on: ubuntu-latest
    # To use Turborepo Remote Caching, set the following environment variables for the job.
    env:
      TURBO_TOKEN: ${{ secrets.TURBO_TOKEN }}
      TURBO_TEAM: ${{ vars.TURBO_TEAM }}

    steps:
      - name: Check out code
        uses: actions/checkout@v4
        with:
          fetch-depth: 2
    # ...
```
