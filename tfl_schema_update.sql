ALTER TABLE tfldata.users DROP COLUMN firebase_user_uid;
CREATE TABLE IF NOT EXISTS tfldata.timecapsule(username varchar(128), available_on date, tcname varchar(18), createdon date);
ALTER TABLE tfldata.users ADD UNIQUE(username);
ALTER TABLE tfldata.timecapsule ADD UNIQUE(tcname);
ALTER TABLE tfldata.users ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.timecapsule ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.calendar ALTER COLUMN event_owner TYPE VARCHAR(15);
ALTER TABLE tfldata.calendar_rsvp ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.catchitleaderboard ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.comments ALTER COLUMN author TYPE VARCHAR(15);
ALTER TABLE tfldata.gchat ALTER COLUMN author TYPE VARCHAR(15);
ALTER TABLE tfldata.posts ALTER COLUMN author TYPE VARCHAR(15);
ALTER TABLE tfldata.reactions ALTER COLUMN author TYPE VARCHAR(15);
ALTER TABLE tfldata.ss_leaderboard ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.stack_leaderboard ALTER COLUMN username TYPE VARCHAR(15);
ALTER TABLE tfldata.threads ALTER COLUMN threadauthor TYPE VARCHAR(15);

ALTER TABLE tfldata.users ADD COLUMN mytz VARCHAR(30);
UPDATE tfldata.users SET mytz='America/New_York' WHERE mytz IS NULL OR mytz='';
ALTER TABLE tfldata.gchat ALTER COLUMN createdon TYPE TIMESTAMPTZ;

ALTER TABLE tfldata.timecapsule ADD COLUMN tcfilename VARCHAR(59);
ALTER TABLE tfldata.timecapsule ADD COLUMN waspurchased bool;
ALTER TABLE tfldata.timecapsule ADD COLUMN wasearlyaccesspurchased bool;
UPDATE tfldata.timecapsule SET wasearlyaccesspurchased=false, waspurchased=false WHERE wasearlyaccesspurchased IS NULL AND waspurchased IS NULL;