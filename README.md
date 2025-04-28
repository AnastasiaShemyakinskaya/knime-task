# knime-task

### Simple App
A small Go service that:

1. Accepts incoming JSON messages over HTTP 
2. Stores messages into a PostgreSQL example database 
3. Publishes them to NATS via lib
4. Retries sending if something fails

### Lib
1. Stores messages into PostgreSQL messages table 
2. Publishes them asynchronously to NATS 
3. Retries unsent messages until successful 
4. Supports leader election to avoid duplicate sending when running multiple instances

### How to Run Locally
1. Prerequisites
   Go 1.21+ 
2. Docker (for Postgres and NATS)
3. docker-compose installed

#### Run application
`make start`

It starts:

* PostgreSQL database 
* NATS messaging server 
* Database migrations service 
* Application service (with 3 replicas)
* NGINX load balancer

#### Rebuild docker applications
`make rebuild`
#### Stop applications
`make stop`

The app will start on http://localhost:8080.

#### Example request
```
POST http://localhost:8080/add
Content-Type: application/json

{
"text": "test"
}
```
**Response**:

* 200 OK – if message was added and published

* 400 Bad Request – if JSON is invalid

* 500 Internal Server Error – if adding fails internally
