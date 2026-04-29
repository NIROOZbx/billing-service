package handlers

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/NIROOZbx/billing-service/internal/domain"
	"github.com/NIROOZbx/billing-service/pkg/apperrors"
	"github.com/NIROOZbx/billing-service/pkg/constants"
	"github.com/NIROOZbx/billing-service/pkg/helpers"
	billingv1 "github.com/NIROOZbx/billing-service/proto"
	"github.com/rs/zerolog"
)

type BillingServer struct {
	billingv1.UnimplementedBillingServiceServer
	usageSvc        domain.UsageService
	subscriptionSvc domain.SubscriptionService
	planSvc         domain.PlanService
	logger          zerolog.Logger
}

func NewBillingServer(usageSvc domain.UsageService, subscriptionSvc domain.SubscriptionService, planSvc domain.PlanService, logger zerolog.Logger) *BillingServer {
	return &BillingServer{
		usageSvc:        usageSvc,
		subscriptionSvc: subscriptionSvc,
		planSvc:         planSvc,
		logger:          logger,
	}
}

// ------ Check & Record Operations ------

func (s *BillingServer) CheckLimit(ctx context.Context, req *billingv1.CheckLimitRequest) (*billingv1.CheckLimitResponse, error) {

	var workspaceID, environmentID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: req.EnvironmentId, Name: "environment id", Dest: &environmentID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("channel", req.Channel).
		Msg("gRPC: CheckLimit requested")

	res, err := s.usageSvc.CheckLimit(ctx, workspaceID, environmentID, req.Channel)
	if err != nil {
		if errors.Is(err, apperrors.ErrLimitReached) && res != nil {
			return &billingv1.CheckLimitResponse{
				Allowed: res.Allowed,
				Reason:  res.Reason,
				Limit:   res.Limit,
				Current: int32(res.Current),
				ResetAt: res.ResetAt.Format("2006-01-02T15:04:05Z07:00"),
			}, mapGRPCError(err)
		}
		if errors.Is(err, apperrors.ErrNoActiveSubscription) {
			return &billingv1.CheckLimitResponse{Allowed: false, Reason: constants.ReasonSubscriptionMissing}, mapGRPCError(err)
		}
		return nil, mapGRPCError(err)
	}

	return &billingv1.CheckLimitResponse{
		Allowed: res.Allowed,
		Reason:  res.Reason,
		Limit:   res.Limit,
		Current: int32(res.Current),
		ResetAt: res.ResetAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (s *BillingServer) RecordUsage(ctx context.Context, req *billingv1.RecordUsageRequest) (*billingv1.RecordUsageResponse, error) {
	var workspaceID, environmentID, configID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: req.EnvironmentId, Name: "environment id", Dest: &environmentID},
		helpers.UUIDField{Value: req.ChannelConfigId, Name: "channel config id", Dest: &configID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("channel", req.Channel).
		Str("provider", req.Provider).
		Bool("success", req.Success).
		Msg("gRPC: RecordUsage requested")

	err := s.usageSvc.RecordUsage(ctx, domain.UpsertProviderUsageInput{
		WorkspaceID:     workspaceID,
		EnvironmentID:   environmentID,
		ChannelConfigID: configID,
		ChannelName:     req.Channel,
		ProviderName:    req.Provider,
		Success:         req.Success,
	})

	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &billingv1.RecordUsageResponse{Acknowledged: true}, nil
}

// ------ Subscription Lifecycle ------

func (s *BillingServer) CreateSubscription(ctx context.Context, req *billingv1.CreateSubscriptionRequest) (*billingv1.CreateSubscriptionResponse, error) {
	s.logger.Info().
		Str("workspace_id", req.WorkspaceId).
		Str("plan_id", req.PlanId).
		Str("payment_provider", req.PaymentProvider).
		Msg("gRPC: CreateSubscription request received")

	var workspaceID, planID uuid.UUID
	var err error

	workspaceID, err = uuid.Parse(req.WorkspaceId)
	if err != nil {
		s.logger.Error().Err(err).Str("workspace_id", req.WorkspaceId).Msg("gRPC: invalid workspace_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid workspace id: %v", err)
	}

	planID, err = uuid.Parse(req.PlanId)
	if err != nil {
		s.logger.Debug().Str("plan_input", req.PlanId).Msg("gRPC: Plan ID is not a UUID, attempting name lookup")
		plan, pErr := s.planSvc.GetPlanByName(ctx, req.PlanId)
		if pErr != nil {
			s.logger.Error().Err(pErr).Str("plan_name", req.PlanId).Msg("gRPC: plan not found by name")
			return nil, status.Errorf(codes.NotFound, "plan '%s' not found: %v", req.PlanId, pErr)
		}
		planID = plan.ID
		s.logger.Debug().Str("plan_name", req.PlanId).Str("resolved_plan_id", planID.String()).Msg("gRPC: plan resolved by name")
	}

	provider := req.PaymentProvider
	if provider == "" {
		provider = "system"
		s.logger.Debug().Msg("gRPC: no payment_provider given, defaulting to 'system'")
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("plan_id", planID.String()).
		Str("payment_provider", provider).
		Msg("gRPC: calling Subscribe")

	subscription, err := s.subscriptionSvc.Subscribe(ctx, domain.CreateSubscriptionInput{
		WorkspaceID:            workspaceID,
		PlanID:                 planID,
		PaymentProvider:        provider,
		ExternalSubscriptionID: req.ExternalSubscriptionId,
		ExternalCustomerID:     req.ExternalCustomerId,
	})
	if err != nil {
		s.logger.Error().Err(err).Str("workspace_id", workspaceID.String()).Msg("gRPC: Subscribe failed")
		return nil, mapGRPCError(err)
	}

	s.logger.Info().
		Str("subscription_id", subscription.ID.String()).
		Str("workspace_id", workspaceID.String()).
		Msg("gRPC: CreateSubscription succeeded")

	return &billingv1.CreateSubscriptionResponse{
		SubscriptionId: subscription.ID.String(),
		Success:        true,
	}, nil
}

func (s *BillingServer) CancelSubscription(ctx context.Context, req *billingv1.CancelSubscriptionRequest) (*billingv1.CancelSubscriptionResponse, error) {
	var workspaceID, subscriptionID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: req.SubscriptionId, Name: "subscription id", Dest: &subscriptionID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("subscription_id", subscriptionID.String()).
		Msg("gRPC: CancelSubscription requested")

	err := s.subscriptionSvc.Cancel(ctx, workspaceID, subscriptionID)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &billingv1.CancelSubscriptionResponse{
		Success: true,
	}, nil
}

// ------ Read Operations ------

func (s *BillingServer) GetSubscription(ctx context.Context, req *billingv1.GetSubscriptionRequest) (*billingv1.GetSubscriptionResponse, error) {
	var workspaceID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Msg("gRPC: GetSubscription requested")

	subscription, err := s.subscriptionSvc.GetActiveSubscription(ctx, workspaceID)
	if err != nil {
		s.logger.Info().Interface("data",subscription).Msg("subsction got ")
		return nil, mapGRPCError(err)
	}


	plan, err := s.planSvc.GetPlanByID(ctx, subscription.PlanID)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &billingv1.GetSubscriptionResponse{
		SubscriptionId:   subscription.ID.String(),
		PlanName:         plan.Name,
		Status:           subscription.Status,
		CurrentPeriodEnd: subscription.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00"),
		PaymentProvider:  subscription.PaymentProvider,
	}, nil
}

func (s *BillingServer) GetUsage(ctx context.Context, req *billingv1.GetUsageRequest) (*billingv1.GetUsageResponse, error) {
	var workspaceID, environmentID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: req.EnvironmentId, Name: "environment id", Dest: &environmentID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("environment_id", environmentID.String()).
		Msg("gRPC: GetUsage requested")

	usageList, err := s.usageSvc.GetUsageSummary(ctx, workspaceID, environmentID)
	if err != nil {
		return nil, mapGRPCError(err)
	}
	sub, err := s.subscriptionSvc.GetActiveSubscription(ctx, workspaceID)
	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return nil, mapGRPCError(err)
	}

	resp := &billingv1.GetUsageResponse{
		WorkspaceId:   workspaceID.String(),
		EnvironmentId: environmentID.String(),
	}

	if sub != nil {
		resp.SubscriptionStatus = sub.Status
		resp.PeriodStart = sub.CurrentPeriodStart.Format("2006-01-02T15:04:05Z07:00")
		resp.PeriodEnd = sub.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00")
	} else {
		resp.SubscriptionStatus = "inactive"
	}

	resp.Usage = make([]*billingv1.ChannelUsage, 0, len(usageList))
	for _, u := range usageList {
		resp.Usage = append(resp.Usage, &billingv1.ChannelUsage{
			ChannelName:  u.ChannelName,
			CurrentUsage: u.CurrentUsage,
		})
	}

	return resp, nil
}

func (s *BillingServer) CreateCheckoutSession(ctx context.Context, req *billingv1.CreateCheckoutSessionRequest) (*billingv1.CreateCheckoutSessionResponse, error) {
	var workspaceID, planID uuid.UUID
	if err := helpers.ParseUUIDs(
		helpers.UUIDField{Value: req.WorkspaceId, Name: "workspace id", Dest: &workspaceID},
		helpers.UUIDField{Value: req.PlanId, Name: "plan id", Dest: &planID},
	); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	s.logger.Info().
		Str("workspace_id", workspaceID.String()).
		Str("plan_id", planID.String()).
		Str("customer_email", req.CustomerEmail).
		Msg("gRPC: CreateCheckoutSession requested")

	url, err := s.subscriptionSvc.CreateCheckoutSession(ctx, workspaceID, planID,req.CustomerEmail)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &billingv1.CreateCheckoutSessionResponse{
		CheckoutUrl: url,
	}, nil
}

func(s *BillingServer)GetCheckoutSession(ctx context.Context,req *billingv1.CreateGetSessionRequest)(*billingv1.GetSessionResponse,error){
	s.logger.Info().
		Str("session_id", req.SessionId).
		Msg("gRPC: GetCheckoutSession requested")

	details, err := s.subscriptionSvc.GetCheckoutSession(ctx, req.SessionId)
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return &billingv1.GetSessionResponse{
		Id:             details.ID,
		CustomerEmail:  details.CustomerEmail,
		AmountTotal:    details.AmountTotal,
		Currency:       details.Currency,
		PaymentStatus:  details.PaymentStatus,
		PlanName:       details.PlanName,
		SubscriptionId: details.SubscriptionID,
	}, nil
}


// ------ Helpers ------

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, apperrors.ErrNoActiveSubscription) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, apperrors.ErrLimitReached) {
		return status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, apperrors.ErrNotFound) || errors.Is(err, apperrors.ErrPlanNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Errorf(codes.Internal, "internal server error: %v", err)
}
