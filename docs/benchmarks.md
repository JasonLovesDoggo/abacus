All Benchmarks are conducted with go run locally, and the redis being ran on the same host as prod.

These numbers were taken using tests/avg.py

## /hit with different TTL implementations (in relation to the data return)
### With expire before

AVG: 66.62284ms, MIN: 53.67ms, MAX: 311.721ms, COUNT: 100

### With expire after

AVG: 64.50046999999999ms, MIN: 53.162ms, MAX: 193.546ms, COUNT: 100

### With expire as a go routine

AVG: 43.408190000000005ms, MIN: 28.913999999999998ms, MAX: 167.731ms, COUNT: 100

### With no expire

AVG: 38.95288ms, MIN: 22.008ms, MAX: 165.397ms, COUNT: 100


