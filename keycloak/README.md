# Using keycloak as IDP

Keycloak is used as an IDP to manage user along with their roles.
Users may use the login button in the iRacelog frontend in order to log in. We use backend for frontend (BFF) so the tokens are kept in the server. The user identification is afterwards done by session cookies.

## Default roles

| Role   | Description                                                 |
| ------ | ----------------------------------------------------------- |
| user   | used to store personal settings                             |
| editor | edit events. allowed tenants should be stored in attributes |
| admin  | no restrictions at all                                      |
