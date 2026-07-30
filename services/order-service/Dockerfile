FROM maven:3.9-eclipse-temurin-21-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY contracts/java ./contracts/java
COPY services ./services

RUN mvn clean package -pl services/order-service -am -DskipTests

FROM eclipse-temurin:21-jre-alpine
RUN adduser -D -u 10001 appuser
USER appuser
WORKDIR /app
COPY --from=builder /app/services/order-service/target/*.jar /app/app.jar

ENV SERVER_PORT=8083
EXPOSE 8083

ENTRYPOINT ["java", "-jar", "/app/app.jar"]
