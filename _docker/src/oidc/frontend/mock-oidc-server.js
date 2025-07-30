
import { OAuth2Server } from 'oauth2-mock-server';

// This is a list of pre-defined users. You can add more users here.
// The filebrowser application will use this data when a user logs in.
const users = {
  // Default user
  johndoe: {
    sub: 'johndoe',
    preferred_username: 'johndoe',
    email: 'johndoe@example.com',
    groups: ['Admins', 'Users'],
  },
  // Another example user
  testuser: {
    sub: 'testuser',
    preferred_username: 'testuser',
    email: 'testuser@example.com',
    groups: ['Users'],
  },
};

const server = new OAuth2Server();

// Generate a new RSA key for the server.
await server.issuer.keys.generate('RS256');

/**
 * This hook is called before the ID token is signed.
 * We're using it to add claims to the ID token.
 * This is the primary way the application will get user information.
 */
server.service.on('beforeIdTokenSigning', (token, req) => {
  // By default, the mock server uses 'johndoe' as the subject.
  const user = users[token.payload.sub] || users.johndoe;
  console.log(`[ID Token] Customizing token for user: ${user.sub}`);
  Object.assign(token.payload, user);
});


/**
 * This hook is called before the userinfo response is sent.
 * We're using it to provide user claims when the application calls the /userinfo endpoint.
 */
server.service.on('beforeUserinfo', (userInfoResponse, req) => {
    const user = users.johndoe;
    console.log(`[UserInfo] Customizing userinfo for user: ${user.sub}`);
    userInfoResponse.body = user;
});


const port = 8080;
const host = '0.0.0.0'; // Bind to 0.0.0.0 to be accessible within Docker
await server.start(port, host);
console.log(`OAuth 2 Mock Server listening on http://${host}:${port}`);
console.log(`Issuer URL: ${server.issuer.url}`);
console.log('---');
console.log('Pre-defined users:');
console.log(JSON.stringify(users, null, 2));
console.log('---');