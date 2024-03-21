import logging

from flask import Flask, request

import uuid
import redis
import requests
from prometheus_client import start_http_server, Summary, Counter

# add explicit otel to demonstrate baggage
from opentelemetry import baggage, context

# setup prom metrics export
start_http_server(9090)
REQUEST_TIME = Summary('request_processing_seconds', 'Time spent processing request')
HEALTH_CHECK_COUNTER = Counter('health_checks', 'Count of health checks')

# connect to redis
r = redis.Redis(host='redis', port=6379, decode_responses=True)

app = Flask(__name__)

@app.route('/health')
def health():
    HEALTH_CHECK_COUNTER.inc()
    return f"KERNEL OK"

@REQUEST_TIME.time()
@app.route('/albums')
def albums():

    logging.getLogger().info("getting albums...")

    # we can explicitly add specific session identifiers to baggage to allow propagation through distributed tracing
    sessionId = r.get(request.remote_addr)
    if sessionId == None:
        r.set(request.remote_addr, uuid.uuid4().hex)
        sessionId = r.get(request.remote_addr)
    context.attach(baggage.set_baggage("sessionId", sessionId))

    error = request.args.get('error')
    if error == None:
        response = requests.get('http://catalog:9000/albums')
        return response.json()
    # allow intentional errors
    elif error == "404":
        logging.getLogger().warn("intentionally getting 404")
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
    app.run(host="0.0.0.0", port=9001, debug=False)