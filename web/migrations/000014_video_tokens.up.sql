-- Convert video quotas from "generations" to "video tokens" (1 generation = 10 tokens)
UPDATE user_quotas SET
    video_weekly = video_weekly * 10,
    video_extra  = video_extra  * 10;
