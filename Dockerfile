FROM alpine:latest

ADD . /home/app/
WORKDIR /home/app

RUN chmod 777 /home/app/Chunzhen

ENTRYPOINT ["./Chunzhen"]
EXPOSE 8080