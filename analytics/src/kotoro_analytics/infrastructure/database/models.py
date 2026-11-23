"""ORM models of the analytics read model.

Tables are projections built from `study_cycle.finished.v1`
(docs/events/study_cycle.finished.v1.md). Column names follow the event payload.
Day buckets (`metric_date`) are calendar days in ANALYTICS_METRICS_TIMEZONE
(see docs/adr/0001-read-database-decisions.md).

Every projection about a person carries `user_id`; every query must filter by it.
"""

import uuid
from datetime import date, datetime

from sqlalchemy import (
    BigInteger,
    CheckConstraint,
    Date,
    DateTime,
    Index,
    Integer,
    MetaData,
    String,
    Uuid,
    func,
)
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column

NAMING_CONVENTION = {
    "ix": "ix_%(table_name)s_%(column_0_N_name)s",
    "uq": "uq_%(table_name)s_%(column_0_N_name)s",
    "ck": "ck_%(table_name)s_%(constraint_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s",
}


class Base(DeclarativeBase):
    metadata = MetaData(naming_convention=NAMING_CONVENTION)


def _created_at() -> Mapped[datetime]:
    return mapped_column(DateTime(timezone=True), nullable=False, server_default=func.now())


def _updated_at() -> Mapped[datetime]:
    return mapped_column(DateTime(timezone=True), nullable=False, server_default=func.now())


class ProcessedEvent(Base):
    """Inbox: one row per applied event, keyed by the envelope `id` (idempotency)."""

    __tablename__ = "processed_events"

    event_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    event_type: Mapped[str] = mapped_column(String(150), nullable=False)
    event_version: Mapped[int] = mapped_column(Integer, nullable=False)
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    processed_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, server_default=func.now()
    )


class StudyCycleFact(Base):
    """One analytical fact per finished study cycle."""

    __tablename__ = "study_cycle_facts"
    __table_args__ = (
        CheckConstraint("elapsed_seconds >= 0", name="elapsed_seconds_non_negative"),
        CheckConstraint("completed_task_count >= 0", name="completed_task_count_non_negative"),
        CheckConstraint("ended_at >= started_at", name="ended_after_started"),
        Index(None, "user_id", "metric_date"),
        Index(None, "sprint_id"),
    )

    cycle_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    event_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False, unique=True)
    user_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)
    sprint_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)
    sprint_block_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)
    subject_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)
    sequence_number: Mapped[int] = mapped_column(Integer, nullable=False)
    execution_mode: Mapped[str] = mapped_column(String(50), nullable=False)
    timer_type: Mapped[str] = mapped_column(String(50), nullable=False)
    stop_reason: Mapped[str] = mapped_column(String(50), nullable=False)
    started_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    ended_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    elapsed_seconds: Mapped[int] = mapped_column(Integer, nullable=False)
    completed_task_count: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    metric_date: Mapped[date] = mapped_column(Date, nullable=False)
    created_at: Mapped[datetime] = _created_at()


class UserDailyMetric(Base):
    """Totals per user and day. Serves summary and daily series by date range."""

    __tablename__ = "user_daily_metrics"
    __table_args__ = (
        CheckConstraint("studied_seconds >= 0", name="studied_seconds_non_negative"),
        CheckConstraint("completed_cycles >= 0", name="completed_cycles_non_negative"),
        CheckConstraint("completed_tasks >= 0", name="completed_tasks_non_negative"),
    )

    user_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    metric_date: Mapped[date] = mapped_column(Date, primary_key=True)
    studied_seconds: Mapped[int] = mapped_column(
        BigInteger, nullable=False, default=0, server_default="0"
    )
    completed_cycles: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    completed_tasks: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    updated_at: Mapped[datetime] = _updated_at()


class UserSubjectDailyMetric(Base):
    """Totals per user, day and subject. Daily grain so subjects can be filtered by period.

    Subject names are not in the event; the frontend resolves them from the main API.
    """

    __tablename__ = "user_subject_metrics"
    __table_args__ = (
        CheckConstraint("studied_seconds >= 0", name="studied_seconds_non_negative"),
        CheckConstraint("completed_cycles >= 0", name="completed_cycles_non_negative"),
        CheckConstraint("completed_tasks >= 0", name="completed_tasks_non_negative"),
    )

    # PK order (user_id, metric_date, subject_id) serves "user X between dates" range scans.
    user_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    metric_date: Mapped[date] = mapped_column(Date, primary_key=True)
    subject_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    studied_seconds: Mapped[int] = mapped_column(
        BigInteger, nullable=False, default=0, server_default="0"
    )
    completed_cycles: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    completed_tasks: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    updated_at: Mapped[datetime] = _updated_at()


class SprintSummary(Base):
    """Running totals of one sprint, built from its finished cycles.

    Sprint status/name are not in the event and are not stored here.
    """

    __tablename__ = "sprint_summaries"
    __table_args__ = (
        CheckConstraint("studied_seconds >= 0", name="studied_seconds_non_negative"),
        CheckConstraint("completed_cycles >= 0", name="completed_cycles_non_negative"),
        CheckConstraint("completed_tasks >= 0", name="completed_tasks_non_negative"),
        Index(None, "user_id"),
    )

    sprint_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    user_id: Mapped[uuid.UUID] = mapped_column(Uuid, nullable=False)
    studied_seconds: Mapped[int] = mapped_column(
        BigInteger, nullable=False, default=0, server_default="0"
    )
    completed_cycles: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    completed_tasks: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    first_cycle_started_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    last_cycle_ended_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    updated_at: Mapped[datetime] = _updated_at()


class UserStreak(Base):
    """Consecutive active days. `current_streak_days` is valid as of `last_active_date`;
    readers must treat it as 0 when `last_active_date` is before yesterday."""

    __tablename__ = "user_streaks"
    __table_args__ = (
        CheckConstraint("current_streak_days >= 0", name="current_streak_non_negative"),
        CheckConstraint(
            "longest_streak_days >= current_streak_days", name="longest_not_below_current"
        ),
    )

    user_id: Mapped[uuid.UUID] = mapped_column(Uuid, primary_key=True)
    current_streak_days: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    longest_streak_days: Mapped[int] = mapped_column(
        Integer, nullable=False, default=0, server_default="0"
    )
    last_active_date: Mapped[date | None] = mapped_column(Date)
    updated_at: Mapped[datetime] = _updated_at()
