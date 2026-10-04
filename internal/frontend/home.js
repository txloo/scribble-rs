// This is the home page's script: a permanent drawing wall and a shared
// chat, both without any websocket. Everything runs over the /v1/hub/*
// HTTP endpoints, which address the room implicitly.
//
// The canvas machinery is extracted from the lobby client; the drawing
// primitives come from the shared draw.js, which is loaded before this
// file and exposes its functions globally.

const rootPath = `{{.RootPath}}`;

// ---------------------------------------------------------------------------
// Canvas setup
// ---------------------------------------------------------------------------

const drawingBoard = document.getElementById("drawing-board");
const contentSide = document.getElementById("content-side");
const chatSide = document.getElementById("chat-side");

// The internal canvas resolution defines the server coordinate space; the
// aspect is 1:1 (square).
const baseWidth = 720;
const baseHeight = 720;

// layoutCanvas sizes the canvas as the largest square that fits in the
// content column above the chat strip. The tool rail is a sibling column,
// so it is excluded automatically; measuring avoids any guessing in CSS,
// which previously made the canvas overflow and cover other elements.
function layoutCanvas() {
    const contentRect = contentSide.getBoundingClientRect();
    const chatRect = chatSide.getBoundingClientRect();

    const availableWidth = contentRect.width;
    const availableHeight = contentRect.height - chatRect.height - 5;

    // A small floor keeps the canvas sane on absurdly narrow windows,
    // where the subtraction could otherwise go zero or negative.
    const canvasSize = Math.max(
        100,
        Math.min(availableWidth, availableHeight),
    );

    drawingBoard.style.width = Math.floor(canvasSize) + "px";
    drawingBoard.style.height = Math.floor(canvasSize) + "px";
}
window.addEventListener("resize", layoutCanvas);

// A ResizeObserver re-fits whenever the content column's size changes,
// including right after the initial layout settles — a plain script-time
// measurement can race with font loading and produce a wrong first fit.
// Loop-safe: the column's own size comes from the flex container, never
// from the canvas' inline size.
const layoutObserver = new ResizeObserver(layoutCanvas);
layoutObserver.observe(contentSide);
layoutCanvas();

// Late-loading fonts change text metrics; re-fit once they are ready.
if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(layoutCanvas);
}

// Moving this here to extract the context after resizing
const context = drawingBoard.getContext("2d", { alpha: false });

// One might one wonder what the fuck is going here. I'll enlighten you!
// The data you put into a canvas, might not necessarily be what comes out
// of it again. Some browser (*cough* firefox *cough*) seem to put little
// off by one / two errors into the data, when reading it back out.
// Apparently this helps against some type of fingerprinting. In order to
// combat this, we do not use the canvas as a source of truth, but
// permanently hold a virtual canvas buffer that we can operate on when
// filling or drawing.
let imageData;

function scaleUpFactor() {
    return baseWidth / drawingBoard.clientWidth;
}

// Will convert the value to the server coordinate space.
// The canvas locally can be bigger or smaller. Depending on the base
// values and the local values, we'll either have a value slightly
// higher or lower than 1.0. Since we draw on a virtual canvas, we have
// to use the server coordinate space, which then gets scaled by the
// canvas API of the browser, as we have a different clientWidth than
// width and clientHeight than height.
function convertToServerCoordinate(value) {
    return Math.round(parseFloat(scaleUpFactor() * value));
}

function clear(context) {
    context.fillStyle = "#FFFFFF";
    context.fillRect(0, 0, drawingBoard.width, drawingBoard.height);
    // Refetch, as we don't manually fill here.
    imageData = context.getImageData(
        0,
        0,
        context.canvas.width,
        context.canvas.height,
    );
}

// Clear initially, as it will be black otherwise.
clear(context);

// ---------------------------------------------------------------------------
// Drawing tools
// ---------------------------------------------------------------------------

const pen = 0;
const rubber = 1;
const fillBucket = 2;

// On the wall everyone may draw at all times.
let allowDrawing = true;

//Initially, we require some values to avoid running into nullpointers
//or undefined errors. The specific values don't really matter.
let localTool = pen;
let localLineWidth = 8;

const toolButtonPen = document.getElementById("tool-type-pencil");
const toolButtonRubber = document.getElementById("tool-type-rubber");
const toolButtonFill = document.getElementById("tool-type-fill");

const sizeSlider = document.getElementById("size-slider");
const sizePreviewDot = document.getElementById("size-preview-dot");

if (toolButtonPen.checked) {
    chooseToolNoUpdate(pen);
} else if (toolButtonFill.checked) {
    chooseToolNoUpdate(fillBucket);
} else if (toolButtonRubber.checked) {
    chooseToolNoUpdate(rubber);
}

let localColor, localColorIndex;

function setColor(index) {
    setColorNoUpdate(index);

    // If we select a new color, we assume we don't want to use the
    // rubber anymore and automatically switch to the pen.
    if (localTool === rubber) {
        // Clicking the button programmatically won't trigger its
        toolButtonPen.click();

        // updateDrawingStateUI is implicit
        chooseTool(pen);
    } else {
        updateDrawingStateUI();
    }
}

const firstColorButtonRow = document.getElementById("first-color-button-row");
const secondColorButtonRow = document.getElementById("second-color-button-row");
for (let i = 0; i < firstColorButtonRow.children.length; i++) {
    const _setColor = () => setColor(i);
    firstColorButtonRow.children[i].addEventListener("mousedown", _setColor);
    firstColorButtonRow.children[i].addEventListener("click", _setColor);
}
for (let i = 0; i < secondColorButtonRow.children.length; i++) {
    const _setColor = () => setColor(i + 13);
    secondColorButtonRow.children[i].addEventListener("mousedown", _setColor);
    secondColorButtonRow.children[i].addEventListener("click", _setColor);
}

function setColorNoUpdate(index) {
    localColorIndex = index;
    localColor = indexToRgbColor(index);
    sessionStorage.setItem("local_color", JSON.stringify(index));
}

setColorNoUpdate(
    JSON.parse(sessionStorage.getItem("local_color")) ?? 13 /* black*/,
);
updateDrawingStateUI();

function setLineWidth(value) {
    setLineWidthNoUpdate(value);
    updateDrawingStateUI();
}

// The slider fires "input" continuously while dragging, which keeps the
// preview dot and cursor circle live.
sizeSlider.addEventListener("input", () => setLineWidth(Number(sizeSlider.value)));

function setLineWidthNoUpdate(value) {
    localLineWidth = value;
}

function chooseTool(value) {
    chooseToolNoUpdate(value);
    updateDrawingStateUI();
}
toolButtonFill.addEventListener("change", () => chooseTool(fillBucket));
toolButtonPen.addEventListener("change", () => chooseTool(pen));
toolButtonRubber.addEventListener("change", () => chooseTool(rubber));
document
    .getElementById("tool-type-fill-wrapper")
    .addEventListener("mouseup", toolButtonFill.click);
document
    .getElementById("tool-type-pencil-wrapper")
    .addEventListener("mouseup", toolButtonPen.click);
document
    .getElementById("tool-type-rubber-wrapper")
    .addEventListener("mouseup", toolButtonRubber.click);
document
    .getElementById("tool-type-fill-wrapper")
    .addEventListener("mousedown", toolButtonFill.click);
document
    .getElementById("tool-type-pencil-wrapper")
    .addEventListener("mousedown", toolButtonPen.click);
document
    .getElementById("tool-type-rubber-wrapper")
    .addEventListener("mousedown", toolButtonRubber.click);

function chooseToolNoUpdate(value) {
    if (value === pen || value === rubber || value === fillBucket) {
        localTool = value;
    } else {
        //If this ends up with an invalid value, we use the pencil.
        localTool = pen;
    }
}

function rgbColorObjectToHexString(color) {
    return (
        "#" +
        numberTo16BitHexadecimal(color.r) +
        numberTo16BitHexadecimal(color.g) +
        numberTo16BitHexadecimal(color.b)
    );
}

function numberTo16BitHexadecimal(number) {
    return Number(number).toString(16).padStart(2, "0");
}

const rubberColor = { r: 255, g: 255, b: 255 };

function updateDrawingStateUI() {
    // Show the active color at the active size in the preview dot, and
    // tint the slider's thumb with the active color.
    sizePreviewDot.style.width = localLineWidth + "px";
    sizePreviewDot.style.height = localLineWidth + "px";
    sizePreviewDot.style.background = rgbColorObjectToHexString(localColor);
    sizeSlider.style.setProperty(
        "--slider-color",
        rgbColorObjectToHexString(localColor),
    );

    updateCursor();
}

function updateCursor() {
    if (localTool === rubber) {
        setCircleCursor(rubberColor, localLineWidth);
    } else {
        setCircleCursor(localColor, localLineWidth);
    }
}

function getComplementaryCursorColor(innerColor) {
    const hsp = Math.sqrt(
        0.299 * (innerColor.r * innerColor.r) +
            0.587 * (innerColor.g * innerColor.g) +
            0.114 * (innerColor.b * innerColor.b),
    );

    if (hsp > 127.5) {
        return { r: 0, g: 0, b: 0 };
    }

    return { r: 255, g: 255, b: 255 };
}

function setCircleCursor(innerColor, size) {
    const outerColor = getComplementaryCursorColor(innerColor);
    const circleSize = size;
    drawingBoard.style.cursor =
        `url('data:image/svg+xml;utf8,` +
        encodeURIComponent(
            `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="32" height="32">` +
                generateSVGCircle(circleSize, innerColor, outerColor) +
                `</svg>')`,
        ) +
        ` ` +
        circleSize / 2 +
        ` ` +
        circleSize / 2 +
        `, auto`;
}

function generateSVGCircle(circleSize, innerColor, outerColor) {
    const circleRadius = circleSize / 2;
    const innerColorCSS =
        "rgb(" + innerColor.r + "," + innerColor.g + "," + innerColor.b + ")";
    const outerColorCSS =
        "rgb(" + outerColor.r + "," + outerColor.g + "," + outerColor.b + ")";
    return (
        `<circle cx="` +
        circleRadius +
        `" cy="` +
        circleRadius +
        `" r="` +
        circleRadius +
        `" style="fill: ` +
        innerColorCSS +
        `; stroke: ` +
        outerColorCSS +
        `;"/>`
    );
}

// ---------------------------------------------------------------------------
// Drawing input and sending
// ---------------------------------------------------------------------------

let lastX = 0;
let lastY = 0;

let touchID = null;

// Strokes are batched: while the pointer is down, every segment is drawn
// locally right away but collected into strokePoints; the whole stroke is
// sent to the server in one request on release. This reduces HTTP chatter
// drastically and makes the server's per-session rate limit a per-stroke
// limit instead of a per-segment one.
let strokeActive = false;
let strokePoints = [];
// While a stroke is in progress, wall refreshes from polling are deferred,
// since a refresh replaces the whole canvas and would erase the in-progress
// stroke visually. The deferred refresh runs right after the stroke is sent.
let pendingWallRefresh = false;

function beginStroke() {
    strokeActive = true;
    strokePoints = [];
}

function flushStroke() {
    if (!strokeActive) {
        return;
    }
    strokeActive = false;

    if (strokePoints.length === 0) {
        return;
    }
    const points = strokePoints;
    strokePoints = [];
    postWallStrokeChain(points);
}

function onMouseDown(event) {
    if (
        allowDrawing &&
        event.pointerType !== "touch" &&
        event.buttons === 1 &&
        localTool !== fillBucket
    ) {
        const clientRect = drawingBoard.getBoundingClientRect();
        lastX = event.clientX - clientRect.left;
        lastY = event.clientY - clientRect.top;

        beginStroke();
    }
}

function pressureToLineWidth(event) {
    //event.button === 0 could be wrong, as it can also be the uninitialized state.
    //Therefore we use event.buttons, which works differently.
    if (
        event.buttons !== 1 ||
        event.pressure === 0 ||
        event.pointerType === "touch"
    ) {
        return 0;
    }
    if (event.pressure === 0.5 || !event.pressure) {
        return localLineWidth;
    }
    return Math.ceil(event.pressure * 32);
}

let lastLineWidth;
function onMouseMove(event) {
    const pressureLineWidth = pressureToLineWidth(event);
    lastLineWidth = pressureLineWidth;

    if (allowDrawing && pressureLineWidth && localTool !== fillBucket) {
        if (!strokeActive) {
            // Covers re-entering the canvas with the button still held:
            // the previous stroke was flushed at the border.
            beginStroke();
        }

        // calculate the offset coordinates based on client mouse position and drawing board client origin
        const clientRect = drawingBoard.getBoundingClientRect();
        const offsetX = event.clientX - clientRect.left;
        const offsetY = event.clientY - clientRect.top;

        // drawing functions must check for context boundaries
        drawLineAndSendEvent(
            context,
            lastX,
            lastY,
            offsetX,
            offsetY,
            pressureLineWidth,
        );
        lastX = offsetX;
        lastY = offsetY;
    }
}

function onMouseLeave(event) {
    if (allowDrawing && lastLineWidth && localTool !== fillBucket) {
        // calculate the offset coordinates based on client mouse position and drawing board client origin
        const clientRect = drawingBoard.getBoundingClientRect();
        const offsetX = event.clientX - clientRect.left;
        const offsetY = event.clientY - clientRect.top;

        // drawing functions must check for context boundaries
        drawLineAndSendEvent(
            context,
            lastX,
            lastY,
            offsetX,
            offsetY,
            lastLineWidth,
        );
        lastX = offsetX;
        lastY = offsetY;
    }

    // Leaving the canvas ends the stroke; re-entering with the button
    // held starts a new one.
    flushStroke();
}

function onMouseClick(event) {
    //event.buttons won't work here, since it's always 0. Since we
    //have a click event, we can be sure that we actually had a button
    //clicked and 0 won't be the uninitialized state.
    if (allowDrawing && event.button === 0) {
        if (localTool === fillBucket) {
            fillAndSendEvent(
                context,
                event.offsetX,
                event.offsetY,
                localColorIndex,
            );
        } else {
            // A single click is a dot: collect the point and flush it as
            // its own stroke, since the click fires after pointerup.
            drawLineAndSendEvent(
                context,
                event.offsetX,
                event.offsetY,
                event.offsetX,
                event.offsetY,
            );
            flushStroke();
        }
    }
}

drawingBoard.addEventListener("pointerdown", onMouseDown);
drawingBoard.addEventListener("pointermove", onMouseMove);
drawingBoard.addEventListener("mouseleave", onMouseLeave);
drawingBoard.addEventListener("click", onMouseClick);

// Releasing the button anywhere ends the stroke, e.g. when the pointer
// leaves the canvas before the release.
window.addEventListener("pointerup", flushStroke);
window.addEventListener("pointercancel", flushStroke);

function onTouchStart(event) {
    //We only allow a single touch
    if (allowDrawing && touchID == null && localTool !== fillBucket) {
        const touch = event.touches[0];
        touchID = touch.identifier;

        // calculate the offset coordinates based on client touch position and drawing board client origin
        const clientRect = drawingBoard.getBoundingClientRect();
        lastX = touch.clientX - clientRect.left;
        lastY = touch.clientY - clientRect.top;

        beginStroke();
    }
}

function onTouchMove(event) {
    // Prevent moving, scrolling or zooming the page
    event.preventDefault();

    if (allowDrawing) {
        for (let i = event.changedTouches.length - 1; i >= 0; i--) {
            if (event.changedTouches[i].identifier === touchID) {
                const touch = event.changedTouches[i];

                // calculate the offset coordinates based on client touch position and drawing board client origin
                const clientRect = drawingBoard.getBoundingClientRect();
                const offsetX = touch.clientX - clientRect.left;
                const offsetY = touch.clientY - clientRect.top;

                // drawing functions must check for context boundaries
                drawLineAndSendEvent(context, lastX, lastY, offsetX, offsetY);
                lastX = offsetX;
                lastY = offsetY;

                return;
            }
        }
    }
}

function onTouchEnd(event) {
    for (let i = event.changedTouches.length - 1; i >= 0; i--) {
        if (event.changedTouches[i].identifier === touchID) {
            touchID = null;
            flushStroke();
            return;
        }
    }
}

drawingBoard.addEventListener("touchend", onTouchEnd);
drawingBoard.addEventListener("touchcancel", onTouchEnd);
drawingBoard.addEventListener("touchstart", onTouchStart);
drawingBoard.addEventListener("touchmove", onTouchMove);

function fillAndSendEvent(context, x, y, colorIndex) {
    const xScaled = convertToServerCoordinate(x);
    const yScaled = convertToServerCoordinate(y);
    const color = indexToRgbColor(colorIndex);
    if (
        floodfillUint8ClampedArray(
            imageData.data,
            xScaled,
            yScaled,
            color,
            imageData.width,
            imageData.height,
        )
    ) {
        context.putImageData(imageData, 0, 0);
        const fillInstruction = {
            type: "fill",
            data: {
                x: xScaled,
                y: yScaled,
                color: colorIndex,
            },
        };
        postWallEvent(fillInstruction.type, fillInstruction.data);
    }
}

function drawLineAndSendEvent(
    context,
    x1,
    y1,
    x2,
    y2,
    lineWidth = localLineWidth,
) {
    const color = localTool === rubber ? rubberColor : localColor;
    const colorIndex = localTool === rubber ? 0 /* white */ : localColorIndex;

    const x1Scaled = convertToServerCoordinate(x1);
    const y1Scaled = convertToServerCoordinate(y1);
    const x2Scaled = convertToServerCoordinate(x2);
    const y2Scaled = convertToServerCoordinate(y2);
    drawLine(
        context,
        imageData,
        x1Scaled,
        y1Scaled,
        x2Scaled,
        y2Scaled,
        color,
        lineWidth,
    );

    // The segment is drawn locally right away, but only collected for the
    // stroke chain; the whole chain is sent when the pointer is released.
    strokePoints.push({
        type: "line",
        data: {
            x: x1Scaled,
            y: y1Scaled,
            x2: x2Scaled,
            y2: y2Scaled,
            color: colorIndex,
            width: lineWidth,
        },
    });
}

// ---------------------------------------------------------------------------
// Chat
// ---------------------------------------------------------------------------

const messageInput = document.getElementById("message-input");
const messageContainer = document.getElementById("message-container");

function appendMessage(styleClass, author, message, translatedContent) {
    if (messageContainer.childElementCount >= 100) {
        messageContainer.removeChild(messageContainer.firstChild);
    }

    const newMessageDiv = document.createElement("div");
    newMessageDiv.classList.add("message");
    if (styleClass) {
        newMessageDiv.classList.add(styleClass);
    }

    if (author !== null && author !== "") {
        const authorNameSpan = document.createElement("span");
        authorNameSpan.classList.add("chat-name");
        authorNameSpan.innerText = author;
        newMessageDiv.appendChild(authorNameSpan);
    }

    const messageSpan = document.createElement("span");
    messageSpan.classList.add("message-content");
    messageSpan.innerText = message;
    newMessageDiv.appendChild(messageSpan);

    // Live translation: the original stays the main text; the translation
    // into the viewer's interface language shows as a small line under it.
    if (translatedContent !== undefined && translatedContent !== null && translatedContent !== "") {
        const translationSpan = document.createElement("span");
        translationSpan.classList.add("message-translation");
        translationSpan.innerText = translatedContent;
        newMessageDiv.appendChild(translationSpan);
    }

    messageContainer.appendChild(newMessageDiv);

    // Automatically scroll down, so the newest message is always visible.
    messageContainer.scrollTop = messageContainer.scrollHeight;
}

//This automatically scrolls down the chat on arrivals of new messages
new MutationObserver(() => {
    messageContainer.scrollTop = messageContainer.scrollHeight;
}).observe(messageContainer, { childList: true });

//Used to restore the last message on arrow up.
let lastMessage = "";

const encoder = new TextEncoder();
function sendMessage(event) {
    if (event.key !== "Enter") {
        return;
    }
    if (!messageInput.value) {
        return;
    }

    // While the backend already checks for message length, we want to
    // prevent the loss of input and omit the event / clear here.
    if (encoder.encode(messageInput.value).length > 10000) {
        appendMessage(
            "system-message",
            '{{.Translation.Get "system"}}',
            '{{.Translation.Get "message-too-long"}}',
        );
        //We keep the messageInput content, since it could've been
        //something important and we don't want the user having to
        //rewrite it. Instead they can send it via some other means
        //or shorten it a bit.
        return;
    }

    postChatMessage(messageInput.value);
    lastMessage = messageInput.value;
    messageInput.value = "";
}

messageInput.addEventListener("keypress", sendMessage);
messageInput.addEventListener("keydown", function (event) {
    if (event.key === "ArrowUp" && messageInput.value.length === 0) {
        messageInput.value = lastMessage;
        const length = lastMessage.length;
        // Postpone selection change onto next event queue loop iteration, as
        // nothing will happen otherwise.
        setTimeout(() => {
            // length+1 is necessary, as the selection wont change if start and
            // end are the same,
            messageInput.setSelectionRange(length + 1, length);
        }, 0);
    }
});

async function postChatMessage(content) {
    const response = await fetch(`${rootPath}/v1/hub/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
    });

    if (response.ok) {
        // Fetch our own message right away instead of waiting for the
        // next poll.
        await pollHub();
    } else if (response.status === 429) {
        // Rate limited: the message was silently dropped. We keep the
        // input content so the user can try again later.
        messageInput.value = lastMessage;
    }
}

// ---------------------------------------------------------------------------
// Socketless polling: chat, drawing wall and presence, all over HTTP.
// ---------------------------------------------------------------------------

let hubChatCursor = 0;
let hubWallVersion = 0;

async function pollHub() {
    try {
        const chatResponse = await fetch(
            `${rootPath}/v1/hub/chat?since=${hubChatCursor}`,
        );
        if (!chatResponse.ok) {
            return;
        }
        const chatData = await chatResponse.json();

        for (const message of chatData.messages) {
            appendMessage("system-message", message.author, message.content,
                message.translatedContent);
        }
        hubChatCursor = chatData.latestId;

        if (chatData.wallVersion !== hubWallVersion) {
            if (strokeActive) {
                // Don't replace the canvas while a stroke is in progress;
                // it would visually erase the unsent stroke.
                pendingWallRefresh = true;
            } else {
                hubWallVersion = chatData.wallVersion;
                await refreshHubWall();
            }
        }
    } catch (error) {
        // A failed poll is retried on the next tick; the page stays usable.
        console.log("Home polling failed:", error);
    }
}

// refreshHubWall replays the whole wall into the canvas. This reproduces
// the exact paths that were drawn, since every mousemove produces its own
// line event.
async function refreshHubWall() {
    const wallResponse = await fetch(`${rootPath}/v1/hub/wall`);
    if (!wallResponse.ok) {
        return;
    }
    const wallData = await wallResponse.json();
    hubWallVersion = wallData.version;

    clear(context);
    wallData.events.forEach((drawElement) => {
        const drawData = drawElement.data;
        if (drawElement.type === "fill") {
            floodfillUint8ClampedArray(
                imageData.data,
                drawData.x,
                drawData.y,
                indexToRgbColor(drawData.color),
                imageData.width,
                imageData.height,
            );
        } else if (drawElement.type === "line") {
            drawLineNoPut(
                context,
                imageData,
                drawData.x,
                drawData.y,
                drawData.x2,
                drawData.y2,
                indexToRgbColor(drawData.color),
                drawData.width,
            );
        } else {
            console.log("Unknown draw element type: " + drawElement.type);
        }
    });

    context.putImageData(imageData, 0, 0);
}

async function postWallEvent(type, data) {
    const response = await fetch(`${rootPath}/v1/hub/wall`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
            type,
            event: { type, data },
        }),
    });
    if (response.ok) {
        // Our own stroke is already on the canvas; bump the version so we
        // don't re-fetch our own stroke on the next poll.
        hubWallVersion++;
    }
}

// postWallStrokeChain sends a whole collected stroke in one request. The
// server expands it into regular line events, so the wall's replay and
// persistence format stays unchanged.
async function postWallStrokeChain(points) {
    const response = await fetch(`${rootPath}/v1/hub/wall`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
            type: "line-chain",
            event: points,
        }),
    });

    if (response.ok) {
        // Our own stroke is already on the canvas; bump the version so we
        // don't re-fetch our own stroke on the next poll.
        hubWallVersion++;
    }
    // On failure or rate limiting the stroke stays only on the local
    // canvas; the next wall refresh shows the true wall again.

    if (pendingWallRefresh) {
        pendingWallRefresh = false;
        await refreshHubWall();
    }
}

window.setInterval(pollHub, 3000);

// Bootstrap the chat log and the wall, so a fresh page shows the recent
// history right away.
pollHub();

// ---------------------------------------------------------------------------
// Undo and clear
// ---------------------------------------------------------------------------

const undoButton = document.getElementById("undo-button");
const clearCanvasButton = document.getElementById("clear-canvas-button");

undoButton.addEventListener("click", async () => {
    const response = await fetch(`${rootPath}/v1/hub/wall/undo`, {
        method: "POST",
    });
    if (response.ok || response.status === 404) {
        // 404 simply means there was nothing of ours to undo; either way
        // the authoritative wall is fetched.
        await refreshHubWall();
    }
});

clearCanvasButton.addEventListener("click", async () => {
    const response = await fetch(`${rootPath}/v1/hub/wall/clear`, {
        method: "POST",
    });
    if (response.ok || response.status === 404) {
        // 404 means the wall was already empty; the refresh is a no-op.
        await refreshHubWall();
    }
});

// ---------------------------------------------------------------------------
// Language switcher
// ---------------------------------------------------------------------------

const uiLanguageSelect = document.getElementById("ui-language-select");
if (uiLanguageSelect) {
    uiLanguageSelect.addEventListener("change", () => {
        const target = new URL(`${rootPath}/set-language`, window.location.href);
        target.searchParams.set("language", uiLanguageSelect.value);
        target.searchParams.set("redirect", "/");
        window.location.assign(target);
    });
}
