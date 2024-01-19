ALTER TABLE tfldata.users DROP COLUMN firebase_user_uid;
CREATE TABLE IF NOT EXISTS tfldata.timecapsule(username varchar(128), available_on date, tcname varchar(18), createdon date);