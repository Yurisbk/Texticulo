import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

const STORAGE_KEY = "texticulo_token";

export type AuthUser = { id: string; email: string } | null;

type AuthContextValue = {
  token: string | null;
  user: AuthUser;
  setSession: (token: string | null, user: AuthUser) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

function readStored(): { token: string | null; user: AuthUser } {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { token: null, user: null };
    const parsed = JSON.parse(raw) as { token: string; user: AuthUser };
    return { token: parsed.token, user: parsed.user };
  } catch {
    return { token: null, user: null };
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => readStored().token);
  const [user, setUser] = useState<AuthUser>(() => readStored().user);

  const setSession = useCallback((t: string | null, u: AuthUser) => {
    setToken(t);
    setUser(u);
    if (t && u) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ token: t, user: u }));
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const logout = useCallback(() => {
    setSession(null, null);
  }, [setSession]);

  const value = useMemo(
    () => ({ token, user, setSession, logout }),
    [token, user, setSession, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside AuthProvider");
  return ctx;
}
