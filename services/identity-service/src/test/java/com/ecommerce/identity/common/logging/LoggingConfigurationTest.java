package com.ecommerce.identity.common.logging;

import static org.assertj.core.api.Assertions.assertThat;

import java.io.IOException;
import org.junit.jupiter.api.Test;
import org.springframework.boot.env.YamlPropertySourceLoader;
import org.springframework.core.io.ClassPathResource;

class LoggingConfigurationTest {
    @Test
    void consolePatternIncludesTraceIdFromMdc() throws IOException {
        var sources = new YamlPropertySourceLoader()
                .load("application", new ClassPathResource("application.yml"));

        assertThat(sources)
                .anySatisfy(source -> assertThat(source.getProperty("logging.pattern.console"))
                        .asString()
                        .contains("trace_id=%X{trace_id:-none}"));
    }
}
