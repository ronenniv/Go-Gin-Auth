Mongo

docker run -d --name mongodb -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=password -p 27017:27017 -v $PWD/data:/data/db mongo:latest

Redis

docker run -d -v $PWD/conf:/usr/local/etc/redis --name redis -p 6379:6379 redis:latest /usr/local/etc/redis/redis.conf