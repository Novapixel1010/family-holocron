import { Link } from "react-router-dom";
import { AuthShell } from "./AuthShell";

/**
 * Reserved for identity-provider (OIDC / OAuth2) redirects. Single sign-on isn't
 * available yet, so this explains that and links back to the sign-in screen.
 */
export function AuthCallbackPage() {
  return (
    <AuthShell title="Single sign-on isn’t set up" subtitle="This Family Holocron server doesn’t have an identity provider configured yet.">
      <Link to="/auth" className="block rounded-lg bg-brand-600 px-3.5 py-2 text-center text-sm font-medium text-white hover:bg-brand-700">
        Sign in with your username or email
      </Link>
    </AuthShell>
  );
}
