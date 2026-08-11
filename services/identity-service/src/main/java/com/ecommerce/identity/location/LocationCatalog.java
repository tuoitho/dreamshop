package com.ecommerce.identity.location;

import java.io.IOException;
import java.io.InputStream;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.io.ClassPathResource;
import org.springframework.core.io.Resource;
import org.springframework.stereotype.Component;
import tools.jackson.databind.ObjectMapper;

/**
 * Immutable in-memory lookup over the vendored administrative snapshot
 * {@code locations/vietnam-administrative-units.json}, loaded once at startup.
 *
 * <p>The snapshot is a source-controlled versioned file; the service never calls an external
 * administrative-divisions API at runtime.
 */
@Component
public class LocationCatalog {
    private final Map<String, Province> provincesByCode;

    @Autowired
    public LocationCatalog(ObjectMapper objectMapper) throws IOException {
        this(objectMapper, new ClassPathResource("locations/vietnam-administrative-units.json"));
    }

    LocationCatalog(ObjectMapper objectMapper, Resource resource) throws IOException {
        try (InputStream input = resource.getInputStream()) {
            Province[] provinces = objectMapper.readValue(input, Province[].class);
            Map<String, Province> byCode = new LinkedHashMap<>();
            for (Province province : provinces) {
                if (byCode.put(province.code(), province) != null) {
                    throw new IllegalStateException("duplicate province code " + province.code());
                }
                Set<String> wardCodes = new LinkedHashSet<>();
                for (Ward ward : province.wards()) {
                    if (!wardCodes.add(ward.code())) {
                        throw new IllegalStateException(
                                "duplicate ward code " + ward.code() + " in province " + province.code());
                    }
                }
            }
            this.provincesByCode = Collections.unmodifiableMap(byCode);
        }
    }

    /** All provinces in snapshot order, as an unmodifiable list. */
    public List<Province> provinces() {
        return List.copyOf(provincesByCode.values());
    }

    /** The province with the given code, if present in the snapshot. */
    public Optional<Province> province(String code) {
        return Optional.ofNullable(provincesByCode.get(code));
    }

    /** True when the ward code belongs to the given province code in the snapshot. */
    public boolean wardBelongsToProvince(String provinceCode, String wardCode) {
        Province province = provincesByCode.get(provinceCode);
        if (province == null) {
            return false;
        }
        for (Ward ward : province.wards()) {
            if (ward.code().equals(wardCode)) {
                return true;
            }
        }
        return false;
    }
}
