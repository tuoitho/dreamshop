package com.ecommerce.identity.location.api;

import com.ecommerce.identity.location.LocationCatalog;
import com.ecommerce.identity.location.Province;
import com.ecommerce.identity.location.ProvinceNotFoundException;
import java.util.List;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class LocationController {
    private final LocationCatalog catalog;

    @GetMapping("/api/v1/locations/provinces")
    List<ProvinceResponse> provinces() {
        return catalog.provinces().stream()
                .map(province -> new ProvinceResponse(province.code(), province.name()))
                .toList();
    }

    @GetMapping("/api/v1/locations/provinces/{provinceCode}/wards")
    List<WardResponse> wards(@PathVariable("provinceCode") String provinceCode) {
        Province province = catalog.province(provinceCode).orElseThrow(ProvinceNotFoundException::new);
        return province.wards().stream()
                .map(ward -> new WardResponse(ward.code(), ward.name()))
                .toList();
    }
}
