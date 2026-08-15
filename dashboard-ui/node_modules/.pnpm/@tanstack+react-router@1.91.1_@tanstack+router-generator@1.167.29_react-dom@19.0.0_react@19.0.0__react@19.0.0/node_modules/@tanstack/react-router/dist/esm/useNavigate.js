import * as React from "react";
import { useRouter } from "./useRouter.js";
function useNavigate(_defaultOpts) {
  const { navigate } = useRouter();
  return React.useCallback(
    (options) => {
      return navigate({
        ...options
      });
    },
    [navigate]
  );
}
function Navigate(props) {
  const { navigate } = useRouter();
  React.useEffect(() => {
    navigate({
      ...props
    });
  }, []);
  return null;
}
export {
  Navigate,
  useNavigate
};
//# sourceMappingURL=useNavigate.js.map
