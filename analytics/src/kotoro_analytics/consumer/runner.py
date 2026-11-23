"""Analytics Consumer process: RabbitMQ -> projections in the analytics database.

Delivery is at-least-once (the relay may publish twice); the projector makes it
effectively-once with `processed_events`. A message is acknowledged only after the
projection transaction commits, so a crash before that means redelivery, never loss.
"""

import asyncio
import logging
from zoneinfo import ZoneInfo

import aio_pika
from aio_pika.abc import AbstractIncomingMessage, AbstractQueue, AbstractRobustConnection
from sqlalchemy.ext.asyncio import create_async_engine

from kotoro_analytics.config import Settings
from kotoro_analytics.consumer.handler import (
    HANDLED_ROUTING_KEY,
    Outcome,
    Projector,
    handle_message,
)
from kotoro_analytics.infrastructure.database.projections import StudyCycleFinishedProjector

logger = logging.getLogger(__name__)


async def declare_topology(
    channel: aio_pika.abc.AbstractChannel, settings: Settings
) -> AbstractQueue:
    """Same declarations as the relay, so either process can start first."""
    exchange = await channel.declare_exchange(
        settings.rabbitmq_exchange, aio_pika.ExchangeType.TOPIC, durable=True
    )
    queue = await channel.declare_queue(settings.rabbitmq_queue, durable=True)
    await queue.bind(exchange, routing_key=HANDLED_ROUTING_KEY)
    return queue


def make_callback(projector: Projector, retry_delay_seconds: float):
    async def on_message(message: AbstractIncomingMessage) -> None:
        outcome = await handle_message(
            body=message.body,
            routing_key=message.routing_key,
            message_id=message.message_id,
            redelivered=bool(message.redelivered),
            projector=projector,
        )
        if outcome is Outcome.ACK:
            await message.ack()
        elif outcome is Outcome.REJECT:
            await message.reject(requeue=False)
        else:
            # Holding the message for a while is the backoff: no hot loop while the DB is down.
            await asyncio.sleep(retry_delay_seconds)
            await message.nack(requeue=True)

    return on_message


async def run_consumer(settings: Settings, stop: asyncio.Event) -> None:
    engine = create_async_engine(
        settings.database_url,
        pool_size=settings.db_pool_size,
        max_overflow=settings.db_max_overflow,
        pool_pre_ping=True,
        connect_args={"timeout": settings.db_connect_timeout_seconds},
    )
    projector = StudyCycleFinishedProjector(engine, timezone=ZoneInfo(settings.metrics_timezone))
    connection: AbstractRobustConnection | None = None
    try:
        # connect_robust reconnects and re-declares the topology after a broker restart.
        connection = await aio_pika.connect_robust(settings.rabbitmq_url)
        channel = await connection.channel()
        await channel.set_qos(prefetch_count=settings.rabbitmq_prefetch)
        queue = await declare_topology(channel, settings)

        consumer_tag = await queue.consume(
            make_callback(projector, settings.consumer_retry_delay_seconds)
        )
        logger.info(
            "consuming %s (exchange %s, key %s)",
            settings.rabbitmq_queue,
            settings.rabbitmq_exchange,
            HANDLED_ROUTING_KEY,
        )
        await stop.wait()

        logger.info("stopping: no new deliveries; unacknowledged messages return to the queue")
        await queue.cancel(consumer_tag)
    finally:
        if connection is not None:
            await connection.close()
        await engine.dispose()
        logger.info("consumer stopped")
