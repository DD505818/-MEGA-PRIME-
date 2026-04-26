'use client';

import { useSession } from 'next-auth/react';

export const useAuth = () => useSession();
