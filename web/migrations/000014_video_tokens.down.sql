-- Revert video tokens back to generations
UPDATE user_quotas SET
    video_weekly = video_weekly / 10,
    video_extra  = video_extra  / 10;
