FROM alpine:latest
WORKDIR /app
RUN echo "Hello World" > /app/text.txt
CMD ["cat", "/app/text.txt"]
