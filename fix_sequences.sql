-- Fix web_sessions sequence
SELECT setval('web_sessions_id_seq', (SELECT COALESCE(MAX(id), 1) FROM web_sessions));

-- Fix other sequences just in case
SELECT setval('generation_requests_id_seq', (SELECT COALESCE(MAX(id), 1) FROM generation_requests));
SELECT setval('payments_id_seq', (SELECT COALESCE(MAX(id), 1) FROM payments));
SELECT setval('users_id_seq', (SELECT COALESCE(MAX(id), 1) FROM users));
SELECT setval('user_quotas_id_seq', (SELECT COALESCE(MAX(id), 1) FROM user_quotas));
