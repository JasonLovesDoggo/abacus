# Abacus—A highly scalable and stateless counting API
                                                               
> Note: Abacus was designed as a direct replacement/upgrade for [CountAPI](https://countapi.xyz/) as it got taken down

In order to get started, please visit the docs at https://abacus.jasoncameron.dev

<!--
## Key features
- Blazing-Fast Performance: Powered by Golang and Valkey (fork of redis), Abacus delivers unparalleled speed and efficiency.
- JSONP Support: Seamlessly integrate Abacus into your web applications with cross-origin resource sharing (CORS) support.
-->


<br/>

---
          
### Development

1. Install Golang & Redis

2. Run `go mod install` to install the dependencies
                                                   
3. Add a `.env` file to the root of the project (or set the environment variables manually) following the format specified in .env.example

4. Run `air` (if installed) or `go run .` to build and run the API locally.

5. The API will be running on `http://0.0.0.0:8080` by default.
 

## Todos

- [x] Documentation (https://abacus.jasoncameron.dev)
- [x] ~~K8 Deployment~~ (Render + Redis on OCI)
- [x] JSONP Support (https://gin-gonic.com/docs/examples/jsonp/)
- [x] impl /create endpoint which creates a new counter initialized to 0 and returns a secret key that can be used to modify the counter via the following endpoints
  - [x] /delete endpoint
  - [x] /set endpoint 
  - [x] /reset (alias to /set 0)
  - [x] /update endpoint (updates the counter x)
- [x] SSE Stream for the counters? Low priority.
- [x] Tests
- [x] Rate limiting (max 30 requests per 3 second per IP address)
- [ ] Create [Python](https://github.com/BenJetson/py-countapi), [JS Wrappers](https://github.com/mlomb/countapi-js) & Go client libraries

