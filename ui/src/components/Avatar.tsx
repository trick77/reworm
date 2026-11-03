import { AVATARS, avatarInfo, avatarUrl } from "../avatars";
import { cx } from "../lib/cx";

type AvatarProps = {
  avatar: string;
  size?: number;
  // selected: the picker's ring in the avatar's own colour
  selected?: boolean;
  alt?: string;
  className?: string;
};

export function Avatar({
  avatar,
  size = 38,
  selected,
  alt,
  className,
}: AvatarProps) {
  const info = avatarInfo(avatar);
  const img = (
    <img
      className={cx("avatar", className)}
      src={avatarUrl(avatar)}
      width={size}
      height={size}
      alt={alt ?? info.name}
      draggable={false}
    />
  );
  if (!selected) return img;
  return (
    <span
      className="avatar-pick on"
      style={{ boxShadow: `0 0 0 3px var(--ink),0 0 0 6px ${info.bg}` }}
    >
      {img}
    </span>
  );
}

// AvatarPicker is the profile screen's grid: every avatar with its name, the chosen one ringed.
export function AvatarPicker({
  value,
  onChange,
  size = 56,
}: {
  value: string;
  onChange: (key: string) => void;
  size?: number;
}) {
  return (
    <div className="avatar-grid" role="radiogroup" aria-label="Avatar">
      {AVATARS.map((a) => {
        const on = a.key === value;
        return (
          <button
            key={a.key}
            type="button"
            role="radio"
            aria-checked={on}
            className={cx("avatar-opt", on && "on")}
            onClick={() => onChange(a.key)}
          >
            <Avatar avatar={a.key} size={size} selected={on} alt="" />
            <span className="nm">{a.name}</span>
          </button>
        );
      })}
    </div>
  );
}
