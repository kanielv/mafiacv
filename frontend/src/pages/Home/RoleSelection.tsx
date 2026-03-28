import { useState } from "react";
import { Button, Group, Text } from "@mantine/core";

interface RoleSelectionProps {
  lobbyId: string;
  playerCount: number;
  onChange: (roles: Record<string, number>) => void;
}

const RoleSelection: React.FC<RoleSelectionProps> = ({
  playerCount,
  onChange,
}) => {
  const [mafia, setMafia] = useState(0);
  const [medic, setMedic] = useState(0);
  const [sheriff, setSheriff] = useState(0);
  const [jester, setJester] = useState(0);

  const total = mafia + medic + sheriff + jester;
  const townCount = Math.max(playerCount - total, 0);

  const update = (
    role: string,
    delta: number,
    current: number,
    setter: (n: number) => void,
    snapshot: Record<string, number>
  ) => {
    const next = Math.max(current + delta, 0);
    setter(next);
    onChange({ ...snapshot, [role]: next });
  };

  const snapshot = { mafia, medic, sheriff, jester };

  const roles = [
    { label: "Mafia", key: "mafia", count: mafia, setter: setMafia },
    { label: "Medic", key: "medic", count: medic, setter: setMedic },
    { label: "Sheriff", key: "sheriff", count: sheriff, setter: setSheriff },
    { label: "Jester", key: "jester", count: jester, setter: setJester },
  ];

  return (
    <div>
      {roles.map(({ label, key, count, setter }) => (
        <Group key={key}>
          <Text>{label}</Text>
          <Text>{count}</Text>
          <Button
            onClick={() => update(key, 1, count, setter, snapshot)}
            size="xs"
            disabled={total >= playerCount}
          >
            +
          </Button>
          <Button
            onClick={() => update(key, -1, count, setter, snapshot)}
            size="xs"
            disabled={count === 0}
          >
            -
          </Button>
        </Group>
      ))}

      <Text>Town: {townCount}</Text>
    </div>
  );
};

export default RoleSelection;
