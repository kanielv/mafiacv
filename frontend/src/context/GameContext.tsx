import { createContext, useContext, useEffect, useRef, useState } from "react";
import { sendEvent, onEvent, offEvent } from '../socket';
import { Player } from "../models/player";
import { useNavigate } from "react-router-dom";

type RoleConfig = Record<string, number>;

export interface Narration {
    storyType: string;
    story: string;
    round: number;
}

export interface SheriffResult {
    targetId: string;
    role: string;
    round: number;
}

export type Phase = 'lobby' | 'intro' | 'night' | 'day' | 'nomination' | 'defense' | 'vote' | 'vote-recap' | 'ended';

export interface Nominee {
    id: string;
    name: string;
}

export interface VoteResult {
    nomineeId: string;
    nomineeName: string;
    eliminated: boolean;
    yesVotes: number;
    noVotes: number;
}

interface GameContextValue {
    lobbyId: string | null;
    isHost: boolean;
    players: Player[];
    name: string;
    setName: (n: string) => void;
    role: string | null;
    roleConfig: RoleConfig;
    setRoleConfig: React.Dispatch<React.SetStateAction<RoleConfig>>;
    theme: string;
    setTheme: (t: string) => void;
    narration: Narration | null;
    phase: Phase;
    round: number;
    nightSubmitted: boolean;
    nominationSubmitted: boolean;
    voteSubmitted: boolean;
    sheriffResult: SheriffResult | null;
    dayStartAlive: Record<string, boolean>;
    nominee: Nominee | null;
    voteResult: VoteResult | null;
    winner: string | null;
    resetForHome: () => void;
    createLobby: () => void;
    joinLobby: (code: string) => void;
    startGame: () => void;
    submitNightAction: (action: 'kill' | 'investigate' | 'protect', targetId: string) => void;
    triggerNightTransition: () => void;
    endDiscussion: () => void;
    submitNomination: (targetId: string) => void;
    submitDayVote: (vote: boolean) => void;
    endVoteRecap: () => void;
}

const GameContext = createContext<GameContextValue | null>(null);

// Linger on the Night screen after the server resolves the night so the
// last actor to submit (especially sheriff) can read their result before
// the Day page takes over.
const DAY_TRANSITION_DELAY_MS = 5000;

export function GameProvider({ children }: { children: React.ReactNode }) {
    const navigate = useNavigate();

    const [lobbyId, setLobbyId] = useState<string | null>(null);
    const [isHost, setIsHost] = useState<boolean>(false);
    const [players, setPlayers] = useState<Player[]>([]);
    const [name, setName] = useState("");
    const [role, setRole] = useState<string | null>(null);
    const [roleConfig, setRoleConfig] = useState<RoleConfig>({});
    const [theme, setTheme] = useState<string>("");
    const [narration, setNarration] = useState<Narration | null>(null);
    const [phase, setPhase] = useState<Phase>('lobby');
    const [round, setRound] = useState<number>(0);
    const [nightSubmitted, setNightSubmitted] = useState<boolean>(false);
    const [nominationSubmitted, setNominationSubmitted] = useState<boolean>(false);
    const [voteSubmitted, setVoteSubmitted] = useState<boolean>(false);
    const [sheriffResult, setSheriffResult] = useState<SheriffResult | null>(null);
    const [dayStartAlive, setDayStartAlive] = useState<Record<string, boolean>>({});
    const [nominee, setNominee] = useState<Nominee | null>(null);
    const [voteResult, setVoteResult] = useState<VoteResult | null>(null);
    const [winner, setWinner] = useState<string | null>(null);

    const lobbyIdRef = useRef<string | null>(null);
    const playersRef = useRef<Player[]>([]);
    const roundRef = useRef<number>(0);
    const introTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const dayTransitionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    useEffect(() => { lobbyIdRef.current = lobbyId; }, [lobbyId]);
    useEffect(() => { playersRef.current = players; }, [players]);
    useEffect(() => { roundRef.current = round; }, [round]);

    useEffect(() => {
        const onConnected = (data: { socketId: string }) => console.log('socket:', data.socketId);
        const onLobbyCreated = (data: { lobbyId: string; players: Player[] }) => {
            setLobbyId(data.lobbyId);
            setIsHost(true);
            setPlayers(data.players);
            navigate('/lobby');
        };

        const onPlayersUpdated = (data: { players: Player[] }) => setPlayers(data.players);
        const onGameStarted = () => console.log('game started');

        const onUserDisconnected = (data: { socketId: string }) =>
            setPlayers(prev => prev.filter(p => p.socketID !== data.socketId));

        const onRolesAssigned = (data: { role: string }) => {
            setRole(data.role);
            setPhase('intro');
            setRound(1);
            navigate('/game');
        };

        const onStoryNarration = (data: Narration) => {
            if (data.storyType === 'game_intro') {
                setNarration(data);
                return;
            }
            if (data.storyType === 'night_recap') {
                setNarration(data);
                return;
            }
            if (data.storyType === 'vote_recap') {
                setNarration(data);
                return;
            }
            if (data.storyType === 'game_ending') {
                setNarration(data);
                return;
            }
            console.log('story-narration (unhandled type):', data.storyType);
        };

        const onPhaseChanged = (data: {
            phase: Phase;
            round: number;
            nomineeId?: string;
            nomineeName?: string;
            voteResult?: VoteResult;
        }) => {
            setPhase(data.phase);
            setRound(data.round);
            if (data.phase === 'day') {
                // Snapshot alive state *before* the server's subsequent
                // players-updated applies night deaths, so the Day page can
                // keep showing everyone as alive until the recap finishes.
                setDayStartAlive(
                    Object.fromEntries(playersRef.current.map(p => [p.socketID, p.isAlive]))
                );
                // Hold on the Night view for a beat so the last role to
                // confirm (especially sheriff) can read their result.
                if (dayTransitionTimerRef.current) clearTimeout(dayTransitionTimerRef.current);
                dayTransitionTimerRef.current = setTimeout(() => {
                    setNightSubmitted(false);
                    setSheriffResult(null);
                    navigate('/Day');
                }, DAY_TRANSITION_DELAY_MS);
                return;
            }
            if (data.phase === 'nomination') {
                setNominationSubmitted(false);
                setNominee(null);
                setVoteResult(null);
                navigate('/Nomination');
                return;
            }
            if (data.phase === 'defense') {
                if (data.nomineeId && data.nomineeName) {
                    setNominee({ id: data.nomineeId, name: data.nomineeName });
                }
                navigate('/Defense');
                return;
            }
            if (data.phase === 'vote') {
                setVoteSubmitted(false);
                if (data.nomineeId && data.nomineeName) {
                    setNominee({ id: data.nomineeId, name: data.nomineeName });
                }
                navigate('/Vote');
                return;
            }
            if (data.phase === 'vote-recap') {
                if (data.voteResult) setVoteResult(data.voteResult);
                // Clear any stale night narration so the TypewriterText on
                // the VoteRecap page only renders the vote_recap story.
                setNarration(null);
                navigate('/VoteRecap');
                return;
            }
            if (data.phase === 'night') {
                setNominee(null);
                setNominationSubmitted(false);
                setVoteSubmitted(false);
                setNightSubmitted(false);
                setSheriffResult(null);
                setNarration(null);
                navigate('/night');
                return;
            }
        };

        const onGameEnded = (data: { winner: string; players: Player[] }) => {
            setWinner(data.winner);
            setPhase('ended');
            if (data.players) setPlayers(data.players);
            navigate('/GameOver');
        };

        const onSheriffResult = (data: SheriffResult) => setSheriffResult(data);
        const onActionAcknowledged = () => setNightSubmitted(true);
        const onNominationAcknowledged = () => setNominationSubmitted(true);
        const onVoteAcknowledged = () => setVoteSubmitted(true);

        onEvent('connected', onConnected);
        onEvent('lobby-created', onLobbyCreated);
        onEvent('players-updated', onPlayersUpdated);
        onEvent('game-started', onGameStarted);
        onEvent('user-disconnected', onUserDisconnected);
        onEvent('roles-assigned', onRolesAssigned);
        onEvent('story-narration', onStoryNarration);
        onEvent('phase-changed', onPhaseChanged);
        onEvent('sheriff-result', onSheriffResult);
        onEvent('action-acknowledged', onActionAcknowledged);
        onEvent('nomination-acknowledged', onNominationAcknowledged);
        onEvent('vote-acknowledged', onVoteAcknowledged);
        onEvent('game-ended', onGameEnded);

        return () => {
            offEvent('connected', onConnected);
            offEvent('lobby-created', onLobbyCreated);
            offEvent('players-updated', onPlayersUpdated);
            offEvent('game-started', onGameStarted);
            offEvent('user-disconnected', onUserDisconnected);
            offEvent('roles-assigned', onRolesAssigned);
            offEvent('story-narration', onStoryNarration);
            offEvent('phase-changed', onPhaseChanged);
            offEvent('sheriff-result', onSheriffResult);
            offEvent('action-acknowledged', onActionAcknowledged);
            offEvent('nomination-acknowledged', onNominationAcknowledged);
            offEvent('vote-acknowledged', onVoteAcknowledged);
            offEvent('game-ended', onGameEnded);
            if (introTimerRef.current) clearTimeout(introTimerRef.current);
            if (dayTransitionTimerRef.current) clearTimeout(dayTransitionTimerRef.current);
        };
    }, [navigate]);

    const resetForHome = () => {
        setLobbyId(null);
        setIsHost(false);
        setPlayers([]);
        setRole(null);
        setRoleConfig({});
        setTheme("");
        setNarration(null);
        setPhase('lobby');
        setRound(0);
        setNightSubmitted(false);
        setNominationSubmitted(false);
        setVoteSubmitted(false);
        setSheriffResult(null);
        setDayStartAlive({});
        setNominee(null);
        setVoteResult(null);
        setWinner(null);
        if (introTimerRef.current) clearTimeout(introTimerRef.current);
        if (dayTransitionTimerRef.current) clearTimeout(dayTransitionTimerRef.current);
    };

    const resetGameState = () => {
        setNarration(null);
        setNightSubmitted(false);
        setSheriffResult(null);
        setPhase('lobby');
        setRound(0);
        setRole(null);
    };

    const createLobby = () => {
        resetGameState();
        sendEvent("create-lobby", { name });
    };

    const joinLobby = (code: string) => {
        resetGameState();
        sendEvent("join-lobby", { lobbyId: code, name });
        setLobbyId(code);
        setIsHost(false);
        navigate('/lobby');
    };

    const startGame = () => {
        console.log("Theme: ", theme);
        sendEvent("start-game", { lobbyId, roles: roleConfig, theme });
    };

    const submitNightAction = (action: 'kill' | 'investigate' | 'protect', targetId: string) => {
        if (!lobbyIdRef.current) return;
        sendEvent("night-action", {
            lobbyId: lobbyIdRef.current,
            action,
            targetId,
            round: roundRef.current,
        });
    };

    const triggerNightTransition = () => {
            if (introTimerRef.current) clearTimeout(introTimerRef.current);
            introTimerRef.current = setTimeout(() => {
                setPhase('night');
                navigate('/night');
            }, 5000);
        }

    const endDiscussion = () => {
        if (!lobbyIdRef.current) return;
        sendEvent("end-discussion", { lobbyId: lobbyIdRef.current });
    };

    const submitNomination = (targetId: string) => {
        if (!lobbyIdRef.current) return;
        sendEvent("nominate", {
            lobbyId: lobbyIdRef.current,
            targetId,
            round: roundRef.current,
        });
    };

    const submitDayVote = (vote: boolean) => {
        if (!lobbyIdRef.current) return;
        sendEvent("submit-vote", {
            lobbyId: lobbyIdRef.current,
            vote,
            round: roundRef.current,
        });
    };

    const endVoteRecap = () => {
        if (!lobbyIdRef.current) return;
        sendEvent("end-vote-recap", { lobbyId: lobbyIdRef.current });
    };

    return (
        <GameContext.Provider value={{
            lobbyId, isHost, players, name, setName, role,
            roleConfig, setRoleConfig, theme, setTheme, narration,
            phase, round, nightSubmitted, nominationSubmitted, voteSubmitted,
            sheriffResult, dayStartAlive, nominee, voteResult, winner,
            createLobby, joinLobby, startGame, submitNightAction, triggerNightTransition,
            endDiscussion, submitNomination, submitDayVote, endVoteRecap, resetForHome
        }}>
            {children}
        </GameContext.Provider>
    );
}

export function useGame() {
    const ctx = useContext(GameContext);
    if (!ctx) throw new Error('useGame must be used inside GameProvider');
    return ctx;
}
