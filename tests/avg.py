import requests

COUNT = 100
TIMEOUT = 0.5 #(500ms)
url = 'http://localhost:8080/hit/jasoncameron.dev/'
times = []

for _ in range(COUNT):

    response = requests.get(url, timeout=TIMEOUT)
    times.append(response.elapsed.total_seconds() * 1000)

print(f"AVG: {sum(times)/COUNT}ms, MIN: {min(times)}ms, MAX: {max(times)}ms, COUNT: {len(times)}")
