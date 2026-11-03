// Basic controls from the mock: Card, Button, IconButton, Chip, Segmented, Pill.
import type { ButtonHTMLAttributes, HTMLAttributes, ReactNode } from "react";
import { cx } from "../lib/cx";
import { Icon, type IconName } from "./Icon";

export function Card({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cx("card", className)} {...rest} />;
}

export function Pill({ className, ...rest }: HTMLAttributes<HTMLSpanElement>) {
  return <span className={cx("pill", className)} {...rest} />;
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "sky";
  small?: boolean;
  icon?: IconName;
};

export function Button({
  variant = "primary",
  small,
  icon,
  className,
  children,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cx("btn", variant, small && "small", className)}
      {...rest}
    >
      {icon && <Icon name={icon} />}
      {children}
    </button>
  );
}

export function IconButton({
  icon,
  label,
  className,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  icon: IconName;
  label: string;
}) {
  return (
    <button
      type={type}
      className={cx("icon-btn", className)}
      aria-label={label}
      title={label}
      {...rest}
    >
      <Icon name={icon} />
    </button>
  );
}

export function Chip({
  on,
  count,
  children,
  className,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  on?: boolean;
  count?: ReactNode;
}) {
  return (
    <button
      type={type}
      className={cx("chip", on && "on", className)}
      aria-pressed={on ?? false}
      {...rest}
    >
      {children}
      {count != null && <small>{count}</small>}
    </button>
  );
}

export type SegmentOption<T extends string> = { value: T; label: ReactNode };

export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: readonly SegmentOption<T>[];
  value: T;
  onChange: (v: T) => void;
  label?: string;
}) {
  return (
    <div className="segbar" role="group" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          className={cx(o.value === value && "on")}
          aria-pressed={o.value === value}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
