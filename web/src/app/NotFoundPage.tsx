import { Link } from "react-router-dom";
import { EmptyState } from "@/components/ui/States";
import { PageContainer } from "./Layout";

export function NotFoundPage() {
  return (
    <PageContainer>
      <EmptyState icon="?" title="Page not found" action={<Link to="/" className="text-sm font-medium text-brand-700 underline dark:text-brand-300">Go to the home page</Link>}>
        The page you were looking for doesn’t exist or may have moved.
      </EmptyState>
    </PageContainer>
  );
}
