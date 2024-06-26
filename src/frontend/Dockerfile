FROM node:latest as RECORDSTORE

WORKDIR /app

# Copy package.json and package-lock.json to the working directory
COPY recordstore/package*.json ./
# Install dependencies
RUN npm install

COPY recordstore/ .
RUN npm install @elastic/apm-rum --save

ARG ELASTIC_APM_SERVER_RUM_ENDPOINT
ARG ELASTIC_APM_SERVER_RUM_CREDENTIALS
RUN echo "\
    import { init as initApm } from '@elastic/apm-rum' \n\
    const apm = initApm({ \n\
        serviceName: 'recordstore', \n\
        serverUrl: '${ELASTIC_APM_SERVER_RUM_ENDPOINT}', \n\
        sendCredentials: ${ELASTIC_APM_SERVER_RUM_CREDENTIALS}, \n\
        serviceVersion: '0.1.0', \n\
        environment: 'production' \n\
    }) \n\
    export default apm; \n\
" > src/rum.js
RUN echo "import apm from './rum'\n" | cat - src/index.js > temp && mv temp src/index.js

RUN npm run build

# -------------------------------------
FROM nginx:1.25.3

RUN apt-get update ; apt-get install unzip
ADD https://github.com/open-telemetry/opentelemetry-cpp-contrib/releases/download/webserver%2Fv1.0.4/opentelemetry-webserver-sdk-x64-linux.tgz /opt/opentelemetry-webserver-sdk-x64-linux.tgz.zip
RUN cd /opt ; unzip opentelemetry-webserver-sdk-x64-linux.tgz.zip; tar xvfz opentelemetry-webserver-sdk-x64-linux.tgz
RUN cd /opt/opentelemetry-webserver-sdk; ./install.sh
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/opt/opentelemetry-webserver-sdk/sdk_lib/lib
RUN echo "load_module /opt/opentelemetry-webserver-sdk/WebServerModule/Nginx/1.25.3/ngx_http_opentelemetry_module.so;\n$(cat /etc/nginx/nginx.conf)" > /etc/nginx/nginx.conf
COPY opentelemetry_module.conf /etc/nginx/conf.d

COPY default.conf /etc/nginx/conf.d/default.conf

COPY --from=RECORDSTORE /app/build /usr/share/nginx/html

