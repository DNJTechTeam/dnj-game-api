// Assinatura de JWT de identidade HS256 dentro do próprio k6.
//
// O middleware de autenticação da API (internal/.../middlewares/auth_middlewares.go)
// valida apenas: assinatura HS256 com JWT_IDENTITY_SECRET, iss="dnj-game-api",
// aud contém "dnj-v2" e exp presente. Ele NÃO consulta o banco — o custo de auth
// é puramente CPU (HMAC). Por isso conseguimos forjar tokens localmente para
// milhares de VUs sem tocar no fluxo de e-mail/OTP.
//
// Claims espelham internal/app/services/jwt_service.go:29-38.
import crypto from 'k6/crypto';
import encoding from 'k6/encoding';

const ISSUER = 'dnj-game-api';
const AUDIENCE = 'dnj-v2';

function b64url(str) {
  return encoding.b64encode(str, 'rawurl');
}

// Assina um JWT para um userId (string). ttlSeconds define exp a partir de agora.
export function signIdentityToken(userId, secret, ttlSeconds) {
  const now = Math.floor(Date.now() / 1000);
  const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = b64url(
    JSON.stringify({
      sub: String(userId),
      iss: ISSUER,
      aud: [AUDIENCE],
      iat: now,
      nbf: now,
      exp: now + (ttlSeconds || 3 * 3600),
    }),
  );
  const signingInput = `${header}.${payload}`;
  const signature = crypto.hmac('sha256', secret, signingInput, 'base64rawurl');
  return `${signingInput}.${signature}`;
}
