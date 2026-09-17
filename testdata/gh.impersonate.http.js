const { spawnSync } = require("node:child_process");

client.global.clearAll();

const TOKEN_TTL = 60 * 60; // 1 hour
const BEARER_CACHE_RAW = client.global.get("BEARER_TOKEN_CACHE");
let BEARER_TOKEN_CACHE = BEARER_CACHE_RAW ? JSON.parse(BEARER_CACHE_RAW) : {};

const env = request.environment.get("ENV_NAME");
let BEARER_TOKEN = BEARER_TOKEN_CACHE[env];

let RES;

// Check if we have a valid token cached
if (BEARER_TOKEN) {
  const now = Math.floor(Date.now() / 1000);
  if (now < BEARER_TOKEN.expiry) {
    // Use cached token
    // Set the token in global variable
    client.global.set("BEARER_TOKEN", BEARER_TOKEN.token);
    $kulala.client.global.headers.set(
      "Authorization",
      "Bearer " + BEARER_TOKEN.token,
    );
    return;
  }
}

const whichGH = spawnSync("which", ["gh"], {
  encoding: "utf-8",
});
if (whichGH.status !== 0) {
  client.log("GH CLI is not installed. Please install it to continue.");
  $kulala.request.abort();
  return;
}

// We need to get a new token
RES = spawnSync("gh", ["auth", "token"], {
  encoding: "utf-8",
});

if (RES.error) {
  client.log(RES.error);
  $kulala.request.abort();
  return;
}

const TOKEN = RES.stdout.replace(/\n$/, "");

// Cache the token with its expiry time
const NOW = Math.floor(Date.now() / 1000);
if (!BEARER_TOKEN) {
  BEARER_TOKEN = {};
}
BEARER_TOKEN.token = TOKEN;
// Subtract 60 seconds to account for clock skew, or use returned values from the shell out
BEARER_TOKEN.expiry = NOW + TOKEN_TTL - 60;

$kulala.client.global.headers.set("Authorization", "Bearer " + TOKEN);

client.global.set(
  "BEARER_TOKEN_CACHE",
  JSON.stringify({
    ...BEARER_TOKEN_CACHE,
    [env]: JSON.stringify(BEARER_TOKEN),
  }),
);

// Update the global cache
client.global.set("BEARER_TOKEN", TOKEN);
