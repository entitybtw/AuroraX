import { useEffect, useState } from "react";
import {
  isLoggedIn,
  getUserInfo,
  subscribe,
  type UserInfo,
} from "./session";

export function useSession(): { loggedIn: boolean; user: UserInfo | null } {
  const [loggedIn, setLoggedIn] = useState(isLoggedIn);
  const [user, setUser] = useState(getUserInfo);

  useEffect(
    () =>
      subscribe(() => {
        setLoggedIn(isLoggedIn());
        setUser(getUserInfo());
      }),
    [],
  );

  return { loggedIn, user };
}
