package com.ecommerce.order;

import static org.assertj.core.api.Assertions.assertThat;

import com.ecommerce.order.support.PostgresRabbitIntegrationTestSupport;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.core.env.Environment;

class OrderApplicationTest extends PostgresRabbitIntegrationTestSupport {
    @LocalServerPort
    int serverPort;

    @Autowired
    Environment environment;

    @Test
    void isolatesActuatorOnDedicatedManagementPort() throws Exception {
        Integer managementPort = environment.getProperty("local.management.port", Integer.class);
        assertThat(managementPort).isNotNull().isNotEqualTo(serverPort);

        HttpClient client = HttpClient.newHttpClient();

        HttpResponse<String> mainPortHealth = client.send(
                HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + serverPort + "/actuator/health"))
                        .GET()
                        .build(),
                HttpResponse.BodyHandlers.ofString());
        assertThat(mainPortHealth.statusCode()).isEqualTo(404);

        HttpResponse<String> mgmtReadiness = client.send(
                HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + managementPort + "/actuator/health/readiness"))
                        .GET()
                        .build(),
                HttpResponse.BodyHandlers.ofString());
        assertThat(mgmtReadiness.statusCode()).isEqualTo(200);
        assertThat(mgmtReadiness.body()).contains("\"status\":\"UP\"");
    }
}
