package repositories

// listAvatarURLColumn projects users.avatar_url for list endpoints (feed, rankings)
// only when it is an HTTPS URL or a small data URL. Avatars may be stored as base64
// data URLs of several MB; repeated across a page they push the response past the
// 6 MB AWS Lambda payload limit, and the Function URL then fails without CORS headers.
const listAvatarURLColumn = `CASE WHEN users.avatar_url LIKE 'https://%' OR octet_length(users.avatar_url) <= 100000 THEN users.avatar_url END`
