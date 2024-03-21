from flask import Flask, request
import logging

from datetime import datetime, timezone, timedelta
import redis
import requests
from prometheus_client import start_http_server, Summary, Counter

# setup prom metrics export
start_http_server(9090)
REQUEST_TIME = Summary('request_processing_seconds', 'Time spent processing request')
HEALTH_CHECK_COUNTER = Counter('health_checks', 'Count of health checks')

# connect to redis
r = redis.Redis(host='redis', port=6379, decode_responses=True)

app = Flask(__name__)
app.logger.setLevel(logging.INFO)

@app.route('/health')
def health():
    HEALTH_CHECK_COUNTER.inc()
    return f"KERNEL OK"

@REQUEST_TIME.time()
@app.route('/albums')
def albums():

    app.logger.info("getting albums...")

    last_access = r.get(request.remote_addr)
    if last_access is not None:
        app.logger.info(f"{request.remote_addr} last seen @ {last_access}")
    r.set(request.remote_addr, datetime.now(tz=timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'))

    error = request.args.get('error')
    if error == None:
        response = requests.get('http://catalog:9000/albums')
        return response.json()
    # allow intentional errors
    elif error == "404":
        app.logger.warn("intentionally getting 404")
        response = requests.get('http://catalog:9000/junk')
        return response.text
    elif error == "500":
        raise Exception("Unknown exception encountered")
    elif error == "remote401":
        response = requests.get('http://catalog:9000/albums', params={'error': 'remote401'})
        return response.text
    elif error == "remoteLatency":
        response = requests.get('http://catalog:9000/albums', params={'error': 'remoteLatency'})
        return response.text
        
if __name__ == '__main__':
    app.run(host="0.0.0.0", port=9001, debug=True)