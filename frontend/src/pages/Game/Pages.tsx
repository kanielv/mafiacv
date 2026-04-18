import { Navigate } from "react-router-dom";
import {
  Container,
  Paper,
  Stack,
  Title,
  Text,
  Loader,
  Group,
  Badge,
} from "@mantine/core";
import { useGame } from "../../context/GameContext";

export default function Game() {
  const { role, narration } = useGame();

  if (!role) {
    return <Navigate to="/" replace />;
  }

  return (
    <Container size="md" py="xl">
      <Stack gap="lg">
        <Paper shadow="xs" p="md" radius="md" withBorder>
          <Group justify="space-between">
            <Title order={2}>Your Role</Title>
            <Badge size="lg" variant="filled">
              {role}
            </Badge>
          </Group>
        </Paper>

        <Paper shadow="xs" p="lg" radius="md" withBorder>
          <Stack gap="sm">
            <Title order={3}>The Tale Begins</Title>
            {narration ? (
              <Text size="lg" style={{ whiteSpace: "pre-wrap" }}>
                {narration.story}
              </Text>
            ) : (
              <Group gap="sm">
                <Loader size="sm" />
                <Text c="dimmed">The narrator is preparing the tale...</Text>
              </Group>
            )}
          </Stack>
        </Paper>
      </Stack>
    </Container>
  );
}
