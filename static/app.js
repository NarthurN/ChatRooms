const socket = new WebSocket('ws://localhost:8080/ws');

let currentRoom = null;
let isHost = false;
let currentQuestion = 0;

// Элементы интерфейса
const startScreen = document.getElementById('start-screen');
const hostContainer = document.getElementById('host-container');
const playerContainer = document.getElementById('player-container');
const createBtn = document.getElementById('create-btn');
const joinBtn = document.getElementById('join-btn');
const pinInput = document.getElementById('pin-input');
const nameInput = document.getElementById('name-input');
const submitJoin = document.getElementById('submit-join');
const roomPinDisplay = document.getElementById('room-pin');
const playersList = document.getElementById('players-list');
const startBtn = document.getElementById('start-btn');
const gameScreen = document.getElementById('game-screen');
const questionText = document.getElementById('question-text');
const nextBtn = document.getElementById('next-btn');
const playerWaiting = document.getElementById('player-waiting');
const playerGame = document.getElementById('player-game');
const playerQuestion = document.getElementById('player-question');
const playerOptions = document.getElementById('player-options');
const currentQuestionDisplay = document.getElementById('current-question');
const totalQuestionsDisplay = document.getElementById('total-questions');
const resultsScreen = document.getElementById('results-screen');
const playerResults = document.getElementById('player-results');

// Обработчики событий
createBtn.addEventListener('click', () => {
    socket.send(JSON.stringify({ type: 'create' }));
    startScreen.style.display = 'none';
    hostContainer.style.display = 'block';
    isHost = true;
});

joinBtn.addEventListener('click', () => {
    startScreen.style.display = 'none';
    playerContainer.style.display = 'block';
    isHost = false;
});

submitJoin.addEventListener('click', () => {
    const pin = pinInput.value;
    const name = nameInput.value;
    if (pin && name) {
        currentRoom = pin;
        socket.send(JSON.stringify({
            type: 'join',
            pin: pin,
            name: name
        }));
    }
});

startBtn.addEventListener('click', () => {
    socket.send(JSON.stringify({
        type: 'start',
        pin: currentRoom
    }));
});

nextBtn.addEventListener('click', () => {
    currentQuestion++;
    socket.send(JSON.stringify({
        type: 'next_question',
        pin: currentRoom,
        question: currentQuestion
    }));
});

// Обработка сообщений от сервера
socket.onmessage = (event) => {
    const data = JSON.parse(event.data);
    console.log('Получено:', data);

    switch (data.type) {
        case 'created':
            currentRoom = data.pin;
            roomPinDisplay.textContent = data.pin;
            break;

        case 'player_joined':
            const playerElement = document.createElement('div');
            playerElement.textContent = data.name;
            playersList.appendChild(playerElement);
            break;

        case 'joined':
            document.getElementById('join-screen').style.display = 'none';
            playerWaiting.style.display = 'block';
            break;

        case 'question':
            if (isHost) {
                document.getElementById('host-screen').style.display = 'none';
                gameScreen.style.display = 'block';
                questionText.textContent = data.text;
                currentQuestion = data.question;
            } else {
                playerWaiting.style.display = 'none';
                playerGame.style.display = 'block';
                playerQuestion.textContent = data.text;
                playerOptions.innerHTML = '';
                
                data.options.forEach((option, index) => {
                    const optionBtn = document.createElement('button');
                    optionBtn.textContent = option;
                    optionBtn.className = 'option';
                    optionBtn.addEventListener('click', () => {
                        socket.send(JSON.stringify({
                            type: 'answer',
                            pin: currentRoom,
                            answer: index
                        }));
                    });
                    playerOptions.appendChild(optionBtn);
                });
                
                currentQuestionDisplay.textContent = data.question + 1;
                totalQuestionsDisplay.textContent = data.total;
            }
            break;

        case 'game_over':
            if (isHost) {
                gameScreen.style.display = 'none';
                resultsScreen.style.display = 'block';
                resultsScreen.innerHTML = '<h3>Результаты:</h3>';
                data.results.forEach(result => {
                    resultsScreen.innerHTML += `<p>${result.name}: ${result.score} очков</p>`;
                });
            } else {
                playerGame.style.display = 'none';
                playerResults.style.display = 'block';
                playerResults.innerHTML = '<h3>Игра окончена!</h3>';
                data.results.forEach(result => {
                    playerResults.innerHTML += `<p>${result.name}: ${result.score} очков</p>`;
                });
            }
            break;
    }
};

socket.onclose = () => {
    alert('Соединение закрыто');
};