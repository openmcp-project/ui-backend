# ui-backend

[![REUSE status](https://api.reuse.software/badge/github.com/openmcp-project/ui-backend)](https://api.reuse.software/info/github.com/openmcp-project/ui-backend)

## About this project

This is the backend for our [MCP UI](https://github.com/openmcp-project/ui-frontend).
Its a simple proxy server which sits between the UI frontend and the Kubernetes API server.

### Motivation

We want to call the kubernetes api server directly from the browser, but we have several problems preventing us from calling the api from the browser:

- TLS certificate is not signed from a well-known CA
- CORS is not configured most of the time

### Solution

The `ui-backend` server acts like a proxy when talking to the Crate-Cluster or MCPs from the browser.
The browser sends the request to the `ui-backend`, with authorization data and optionally the project, workspace and controlplane name of the MCP in header data.

- If requesting the Crate: The request will get send to the crate cluster with the authorization data in the headers
- If requesting an MCP: The `ui-backend` will call the Crate to get the `kubeconfig` of the MCP and then calls the MCP with that kubeconfig

There are only some modifications done when piping the request to the api server, preventing some headers from going through.

## Requirements and Setup

You need to have a running mcp landscape. Then reference the KUBECONFIG for the backend using the `KUBECONFIG` environment variable.

The backend can be started using:

```bash
go run cmd/server/main.go
```

## Usage

You can reach the backend on port `3000` and the path as you would directly to the api server.

```txt
For example: http://localhost:3000/api/v1/namespaces
```

Put the authorization data in the following headers:

- `X-Client-Certificate-Data`
- `X-Client-Key-Data`

or (for OIDC):

- `Authorization: <token>`

Also configure the api-server you want to call:

- Crate: Add the header `X-Use-Crate-Cluster: true`
- MCP: Add the headers `X-Project-Name`, `X-Workspace-Name` and `X-Control-Plane-Name`

### Parsing JSON

`ui-backend` support jsonpath (kubectl version) and jq (gojq) to parse json before sending it to the client, reducing the data transfered to the client.

Usage:

- JsonPath: Add a header `X-jsonpath` with the jsonpath query
- JQ: Add a header `X-jq` with the jq query

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openmcp-project/ui-backend/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure

If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openmcp-project/ui-backend/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2025 SAP SE or an SAP affiliate company and ui-backend contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmcp-project/ui-backend).
