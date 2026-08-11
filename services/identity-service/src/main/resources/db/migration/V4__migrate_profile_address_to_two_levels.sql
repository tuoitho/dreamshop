alter table identity_profiles add column ward_code varchar(10);
update identity_profiles set province_code = null;
alter table identity_profiles drop column district;
alter table identity_profiles drop column ward;
