import { forwardRef, type ButtonHTMLAttributes } from "react";
import { Spinner } from "./States";

type Variant = "primary" | "secondary" | "ghost" | "danger";

const styles: Record<Variant, string> = {
  primary: "bg-brand-600 text-white hover:bg-brand-700 focus-visible:ring-brand-500 disabled:bg-brand-600/60",
  secondary:
    "bg-white text-stone-800 ring-1 ring-inset ring-stone-300 hover:bg-stone-50 dark:bg-stone-800 dark:text-stone-100 dark:ring-stone-700 dark:hover:bg-stone-700",
  ghost: "text-stone-700 hover:bg-stone-100 dark:text-stone-200 dark:hover:bg-stone-800",
  danger: "bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500 disabled:bg-red-600/60",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: "sm" | "md";
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "md", loading, disabled, className = "", children, type = "button", ...rest },
  ref,
) {
  const sz = size === "sm" ? "px-2.5 py-1.5 text-sm" : "px-3.5 py-2 text-sm";
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={`inline-flex items-center justify-center gap-2 rounded-lg font-medium transition focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 disabled:cursor-not-allowed disabled:opacity-70 ${sz} ${styles[variant]} ${className}`}
      {...rest}
    >
      {loading && <Spinner className="h-4 w-4" />}
      {children}
    </button>
  );
});
