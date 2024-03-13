# Abacus - A simple counting API written in Nim 
                                                               
          
# Installation
1. Install Nim & Redis
2. Run `nimble install` to install the dependencies
3. Run `nim c -r --verbosity:0 src/abacus.nim` to build and run the API locally.
4. The API will be running on `http://localhost:5000` by default.


## Introduction
Abacus is a simple counting API written in Nim. It is a simple REST API that allows you to create, read, update and delete counts. It is a simple project that I created to learn Nim and to get a feel for the language.
I currently use it on my personal website to keep track of the number of visitors.

 

# Todos

- [ ] use md2html instead of redirect



## Stack
- **Nim** - The programming language used to write the API
- **Jester** - The web framework used to create the REST API
- **Redis** - The database used to store the counts
- **Nim/Redis** - The Redis client used to interact with the Redis database
- **Docker** - The containerization tool used to run the API
- **Github Actions** - The CI/CD tool used to build and deploy the API
