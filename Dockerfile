FROM alpine:latest
WORKDIR /app
RUN echo "Hello World" > /app/text.txt
RUN whoami
RUN ls -la
RUN pwd
CMD ["cat", "/app/text.txt"]
