import "@mantine/core/styles.css";
import { Group, Text, Button } from "@mantine/core";
import '../index.css';

type PlayerProps = {
    name: string;
    label?: string;
    selected?: boolean;
    onClick?: () => void;
};

export default function Player({ name, label = "go", selected = false, onClick }: PlayerProps) {
  return (
        <Group
            mb="xs"
            justify="space-between"
            className="rounded-lg"
            style={{
                border: selected ? '4px solid #3E8E7E' : '4px solid #E94560',
                backgroundColor: selected ? 'rgba(62,142,126,0.15)' : 'transparent',
            }}
        >
            <Text size="sm" mb="xs" mt="xs" fw={900} ml="xs" color="#E94560">{name}</Text>
            <Button mr="md" color={selected ? 'teal' : 'red'} onClick={onClick}>
              {label}
            </Button>
        </Group>
  );
}
