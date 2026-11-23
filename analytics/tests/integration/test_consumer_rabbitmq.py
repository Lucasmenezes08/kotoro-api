"""Consumer end to end: real RabbitMQ + real PostgreSQL.

Uses a throwaway exchange and queue (random names, deleted at the end), so the real
`kotoro.analytics.events` queue is never touched. Skipped when RabbitMQ is unreachable
(ANALYTICS_TEST_RABBITMQ_URL, default amqp://guest:guest@localhost:5672/).
"""

import asyncio
import json
import os
import uuid
from collections.abc import AsyncIterator, Awaitable, Callable
from datetime import UTC, datetime

import aio_pika
import pytest
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncEngine

from kotoro_analytics.config import Settings
from kotoro_analytics.consumer.handler import HANDLED_ROUTING_KEY
from kotoro_analytics.consumer.runner import run_consumer
from kotoro_analytics.infrastructure.database.models import ProcessedEvent, UserDailyMetric
from tests.support.events import event_dict, event_json

pytestmark = pytest.mark.anyio

RABBITMQ_URL = os.environ.get("ANALYTICS_TEST_RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
NOON = datetime(2026, 11, 20, 15, 0, tzinfo=UTC)

Publish = Callable[..., Awaitable[None]]


@pytest.fixture
async def broker(migrated_database: str) -> AsyncIterator[tuple[Settings, Publish]]:
    """Runs the real consumer against a private exchange/queue; yields a publish function."""
    try:
        connection = await asyncio.wait_for(aio_pika.connect(RABBITMQ_URL), timeout=3)
    except (OSError, TimeoutError, aio_pika.exceptions.AMQPConnectionError) as exc:
        pytest.skip(f"RabbitMQ not reachable for integration tests ({type(exc).__name__})")

    suffix = uuid.uuid4().hex[:8]
    settings = Settings(
        environment="test",
        database_url=migrated_database,
        rabbitmq_url=RABBITMQ_URL,
        rabbitmq_exchange=f"kotoro.domain-events.test-{suffix}",
        rabbitmq_queue=f"kotoro.analytics.events.test-{suffix}",
        consumer_retry_delay_seconds=0.1,
    )
    channel = await connection.channel()
    exchange = await channel.declare_exchange(
        settings.rabbitmq_exchange, aio_pika.ExchangeType.TOPIC, durable=True
    )

    async def publish(
        body: bytes, *, message_id: str, routing_key: str = HANDLED_ROUTING_KEY
    ) -> None:
        # Same properties as the Go publisher (persistent JSON, MessageId = event id).
        await exchange.publish(
            aio_pika.Message(
                body,
                content_type="application/json",
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
                message_id=message_id,
                type="study_cycle.finished",
            ),
            routing_key=routing_key,
        )

    stop = asyncio.Event()
    consumer = asyncio.create_task(run_consumer(settings, stop))
    await asyncio.sleep(0.5)  # let it declare and bind the queue
    try:
        yield settings, publish
    finally:
        stop.set()
        await asyncio.wait_for(consumer, timeout=10)
        await channel.queue_delete(settings.rabbitmq_queue)
        await exchange.delete()
        await connection.close()


async def _eventually(check: Callable[[], Awaitable[bool]], timeout: float = 10.0) -> None:
    deadline = asyncio.get_running_loop().time() + timeout
    while not await check():
        if asyncio.get_running_loop().time() > deadline:
            raise AssertionError("condition not met in time")
        await asyncio.sleep(0.1)


async def _seconds(engine: AsyncEngine, user_id: uuid.UUID) -> int:
    async with engine.connect() as connection:
        value = await connection.scalar(
            select(func.coalesce(func.sum(UserDailyMetric.studied_seconds), 0)).where(
                UserDailyMetric.user_id == user_id
            )
        )
    return int(value)


async def _queue_size(settings: Settings) -> int:
    connection = await aio_pika.connect(RABBITMQ_URL)
    try:
        channel = await connection.channel()
        queue = await channel.declare_queue(settings.rabbitmq_queue, passive=True)
        return queue.declaration_result.message_count
    finally:
        await connection.close()


async def test_published_event_reaches_the_read_model(
    broker: tuple[Settings, Publish], writer_engine: AsyncEngine
) -> None:
    settings, publish = broker
    user = uuid.uuid4()
    body = event_dict(user, started_at=NOON, elapsed_seconds=1_500)

    await publish(json.dumps(body).encode(), message_id=body["id"])

    async def projected() -> bool:
        return await _seconds(writer_engine, user) == 1_500

    await _eventually(projected)
    await _eventually(lambda: _is_empty(settings))


async def _is_empty(settings: Settings) -> bool:
    return await _queue_size(settings) == 0


async def test_redelivered_message_is_counted_once(
    broker: tuple[Settings, Publish], writer_engine: AsyncEngine
) -> None:
    settings, publish = broker
    user, event_id = uuid.uuid4(), uuid.uuid4()
    body = event_json(user, started_at=NOON, elapsed_seconds=900, event_id=event_id)

    # The relay may publish the same event twice (crash after ACK, before MarkPublished).
    for _ in range(3):
        await publish(body, message_id=str(event_id))

    async def all_consumed() -> bool:
        return await _queue_size(settings) == 0

    await _eventually(all_consumed)
    await asyncio.sleep(0.3)
    assert await _seconds(writer_engine, user) == 900
    async with writer_engine.connect() as connection:
        assert (
            await connection.scalar(
                select(func.count())
                .select_from(ProcessedEvent)
                .where(ProcessedEvent.event_id == event_id)
            )
            == 1
        )


async def test_invalid_message_is_dropped_and_does_not_block_the_queue(
    broker: tuple[Settings, Publish], writer_engine: AsyncEngine
) -> None:
    settings, publish = broker
    user = uuid.uuid4()

    await publish(b'{"not": "an event"}', message_id="garbage")
    await publish(event_json(user, started_at=NOON, elapsed_seconds=300), message_id="valid")

    async def valid_one_projected() -> bool:
        return await _seconds(writer_engine, user) == 300

    await _eventually(valid_one_projected)
    await _eventually(lambda: _is_empty(settings))
