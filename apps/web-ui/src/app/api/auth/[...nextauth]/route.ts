import NextAuth from 'next-auth';
import Credentials from 'next-auth/providers/credentials';

const handler = NextAuth({
  session: { strategy: 'jwt' },
  providers: [
    Credentials({
      name: 'Credentials',
      credentials: { username: {}, password: {} },
      async authorize(credentials) {
        if (credentials?.username) {
          return { id: credentials.username, name: credentials.username, role: 'operator' };
        }
        return null;
      }
    })
  ]
});

export { handler as GET, handler as POST };
