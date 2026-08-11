package com.ecommerce.identity.location;

import java.util.List;

/**
 * One province/city from the vendored administrative snapshot with its ward/commune records.
 *
 * @param code  zero-padded administrative code, for example "79"
 * @param name  display name, for example "Thành phố Hồ Chí Minh"
 * @param wards ward/commune records belonging to this province
 */
public record Province(String code, String name, List<Ward> wards) {
    public Province {
        wards = List.copyOf(wards);
    }
}
