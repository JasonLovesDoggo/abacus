# Abacus - A simple counting API written in Golang 
                                                               
[//]: # (          )
[//]: # (# Installation)

[//]: # (1. Install Nim & Redis)

[//]: # (2. Run `nimble install` to install the dependencies)

[//]: # (3. Run `nim c -r --verbosity:0 src/abacus.nim` to build and run the API locally.)

[//]: # (4. The API will be running on `http://localhost:5000` by default.)


## Introduction
Abacus is a simple counting API written in Golang. It is a simple REST API that allows you to create, read, update and delete counts. It is a simple project that I created to learn ~~Nim~~ Go and to get a feel for the language.
I currently use it on my personal website to keep track of the number of visitors.

 

# Todos

- [ ] Documentation
- [ ] K8 Deployment
- [ ] impl /create endpoint which creates a new counter initialized to 0 and returns a secret key that can be used to modify the counter via the following endpoints
  - [ ] /delete endpoint
  - [ ] /set endpoint 
  - [ ] /reset (alias to /set 0)
  - [ ] /update endpoint (updates the counter x)
- [ ] SSE Stream for the counters? Low priority.
- [ ] Tests
- [ ] Rate limiting (max 30 requests per second per IP address)
- [ ] Create [Python](https://github.com/BenJetson/py-countapi) & [JS Wrappers](https://github.com/mlomb/countapi-js)

