FROM python:3.8-slim-buster

WORKDIR /python-docker

COPY requirements.txt requirements.txt
RUN pip3 install -r requirements.txt

# add OTel libs
RUN pip3 install opentelemetry-distro opentelemetry-exporter-otlp
# add OTel auto-instrumentation libs matching installed Python modules
RUN opentelemetry-bootstrap -a install

COPY app.py .

EXPOSE 9001
CMD [ "python3", "-m" , "flask", "run", "--host=0.0.0.0", "-p", "9001"]