# Web development with Gin
## Authentication with JWT, Auth0, and Cookies

Check the branches for code examples:
* Auth0
* JWT-session
* Cookie session

Check out this Medium article for more details - [Golang-Gin Authentication (Auth0, JWT, Cookie)](https://ronen-niv.medium.com/golang-gin-authentication-auth0-jwt-cookie-1d68d874eb03)


### Mongo

docker run -d --name mongodb -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=password -p 27017:27017 -v $(pwd)/data:/data/db mongo:latest


### Redis

docker run -d -v $(pwd)/conf:/usr/local/etc/redis --name redis -p 6379:6379 redis:latest /usr/local/etc/redis/redis.conf

### Logging
Set LOG_LEVEL to DEBUG, INFO, or none for Error level only