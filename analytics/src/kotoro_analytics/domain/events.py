"""Incoming event contracts. Source of truth: docs/events/study_cycle.finished.v1.md.

Unknown extra fields are ignored so the producer can add optional fields to v1
(backward-compatible change). Anything that breaks the contract is rejected.
"""

import uuid
from typing import Literal, Self

from pydantic import AwareDatetime, BaseModel, ConfigDict, Field, ValidationError, model_validator

STUDY_CYCLE_FINISHED = "study_cycle.finished"


class InvalidEventError(ValueError):
    """The message can never be applied (malformed or a different contract)."""


class StudyCycleFinishedPayloadV1(BaseModel):
    model_config = ConfigDict(extra="ignore", frozen=True)

    cycle_id: uuid.UUID
    sprint_id: uuid.UUID
    sprint_block_id: uuid.UUID
    subject_id: uuid.UUID
    sequence_number: int = Field(ge=0)
    execution_mode: str = Field(min_length=1, max_length=50)
    timer_type: str = Field(min_length=1, max_length=50)
    started_at: AwareDatetime
    ended_at: AwareDatetime
    elapsed_seconds: int = Field(ge=0)
    stop_reason: str = Field(min_length=1, max_length=50)
    completed_task_ids: list[uuid.UUID] = Field(default_factory=list)

    @model_validator(mode="after")
    def _ended_after_started(self) -> Self:
        if self.ended_at < self.started_at:
            raise ValueError("ended_at must not be before started_at")
        return self


class StudyCycleFinishedEventV1(BaseModel):
    """Envelope written by the Go API's outbox and published by the relay."""

    model_config = ConfigDict(extra="ignore", frozen=True)

    id: uuid.UUID
    user_id: uuid.UUID
    aggregate_type: Literal["study_cycle"]
    aggregate_id: uuid.UUID
    event_type: Literal["study_cycle.finished"]
    event_version: Literal[1]
    occurred_at: AwareDatetime
    payload: StudyCycleFinishedPayloadV1
    correlation_id: uuid.UUID | None = None
    causation_id: uuid.UUID | None = None

    @model_validator(mode="after")
    def _aggregate_is_the_cycle(self) -> Self:
        if self.aggregate_id != self.payload.cycle_id:
            raise ValueError("aggregate_id must equal payload.cycle_id")
        return self


def parse_study_cycle_finished(body: bytes | str) -> StudyCycleFinishedEventV1:
    try:
        return StudyCycleFinishedEventV1.model_validate_json(body)
    except ValidationError as exc:
        # Field locations and rule names only; message bodies are not secrets but stay out.
        problems = ", ".join(
            f"{'.'.join(str(part) for part in error['loc']) or 'body'}: {error['type']}"
            for error in exc.errors()
        )
        raise InvalidEventError(problems) from exc
