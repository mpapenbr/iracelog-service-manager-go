BEGIN;
CREATE INDEX IF NOT EXISTS car_proto_rs_info_idx ON car_state_proto (rs_info_id);
CREATE INDEX IF NOT EXISTS speedmap_proto_rs_info_idx ON speedmap_proto (rs_info_id);
CREATE INDEX IF NOT EXISTS msg_proto_rs_info_idx ON msg_state_proto (rs_info_id);
CREATE INDEX IF NOT EXISTS race_state_rs_info_idx ON race_state_proto (rs_info_id);
COMMIT;
