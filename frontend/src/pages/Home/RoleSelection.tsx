import { Button, Group, Text } from "@mantine/core";

interface RoleSelectionProps {
  lobbyId: string;
  playerCount: number;
  value: Record<string, number>;
  onChange: (
    updater: (prev: Record<string, number>) => Record<string, number>
  ) => void;
}

const ROLES = [
  { label: "Mafia", key: "mafia" },
  { label: "Medic", key: "medic" },
  { label: "Sheriff", key: "sheriff" },
  { label: "Jester", key: "jester" },
] as const;

const RoleSelection: React.FC<RoleSelectionProps> = ({
  playerCount,
  value,
  onChange,
}) => {
  const total = ROLES.reduce((sum, r) => sum + (value[r.key] ?? 0), 0);
  const townCount = Math.max(playerCount - total, 0);

  const adjust = (key: string, delta: number) => {
    onChange((prev) => {
      const current = prev[key] ?? 0;
      const next = Math.max(current + delta, 0);
      const others = ROLES.reduce(
        (sum, r) => (r.key === key ? sum : sum + (prev[r.key] ?? 0)),
        0
      );
      if (delta > 0 && others + next > playerCount) return prev;
      return { ...prev, [key]: next };
    });
  };

  return (
    <div>
      {ROLES.map(({ label, key }) => {
        const count = value[key] ?? 0;
        return (
          <Group key={key}>
            <Text>{label}</Text>
            <Text>{count}</Text>
            <Button
              onClick={() => adjust(key, 1)}
              size="xs"
              disabled={total >= playerCount}
            >
              +
            </Button>
            <Button
              onClick={() => adjust(key, -1)}
              size="xs"
              disabled={count === 0}
            >
              -
            </Button>
          </Group>
        );
      })}

      <Text>Town: {townCount}</Text>
    </div>
  );
};

export default RoleSelection;
